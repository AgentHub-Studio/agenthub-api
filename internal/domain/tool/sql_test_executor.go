package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const defaultSQLTestMaxRows = 100
const maxSQLTestRows = 1000

// SQLTestExecutor runs a SQL tool test after its datasource credentials have
// been resolved in the current tenant.
type SQLTestExecutor interface {
	Execute(ctx context.Context, credentials DatasourceCreds, query string, inputs map[string]any, operation string, maxRows int) (string, error)
}

type postgresSQLTestExecutor struct{}

func (s *Service) testSQLTool(ctx context.Context, cfg map[string]any, inputs map[string]any) (string, error) {
	if s.dsRdr == nil || s.tenantIDFn == nil {
		return "", fmt.Errorf("tool: SQL test feature not configured")
	}

	datasourceID, query, operation, maxRows, err := sqlTestConfig(cfg)
	if err != nil {
		return "", err
	}

	credentials, err := s.dsRdr.GetDatasourceCreds(ctx, s.tenantIDFn(ctx), datasourceID)
	if err != nil {
		return "", fmt.Errorf("tool: SQL datasource not found")
	}
	if !strings.EqualFold(credentials.Type, "POSTGRESQL") {
		return "", fmt.Errorf("tool: SQL test supports POSTGRESQL datasource only")
	}

	executor := s.sqlTestExecutor
	if executor == nil {
		executor = postgresSQLTestExecutor{}
	}
	result, err := executor.Execute(ctx, credentials, query, inputs, operation, maxRows)
	if err != nil {
		return "", err
	}
	return redactHTTPToolTestResponseBody(result, cfg, inputs, nil), nil
}

func sqlTestConfig(cfg map[string]any) (uuid.UUID, string, string, int, error) {
	rawDatasourceID, hasCanonical := cfg["datasource_id"]
	rawAlias, hasAlias := cfg["dataSourceId"]
	if hasCanonical && hasAlias && !sameSQLDatasourceID(rawDatasourceID, rawAlias) {
		return uuid.Nil, "", "", 0, fmt.Errorf("tool: conflicting SQL config aliases datasource_id and dataSourceId")
	}
	if !hasCanonical {
		rawDatasourceID = rawAlias
	}
	datasourceString, ok := rawDatasourceID.(string)
	if !ok || strings.TrimSpace(datasourceString) == "" {
		return uuid.Nil, "", "", 0, fmt.Errorf("tool: SQL tool has no datasource configured")
	}
	datasourceID, err := uuid.Parse(strings.TrimSpace(datasourceString))
	if err != nil {
		return uuid.Nil, "", "", 0, fmt.Errorf("tool: SQL datasource ID is invalid")
	}

	query, _ := cfg["query"].(string)
	if strings.TrimSpace(query) == "" {
		query, _ = cfg["sql"].(string)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return uuid.Nil, "", "", 0, fmt.Errorf("tool: SQL tool has no query configured")
	}

	operation, _ := cfg["operation"].(string)
	operation = strings.ToUpper(strings.TrimSpace(operation))
	if operation == "" {
		operation = "SELECT"
	}
	switch operation {
	case "SELECT", "INSERT", "UPDATE", "DELETE":
	default:
		return uuid.Nil, "", "", 0, fmt.Errorf("tool: unsupported SQL operation %q", operation)
	}

	maxRows := defaultSQLTestMaxRows
	if rawMaxRows, exists := cfg["max_rows"]; exists {
		value, ok := integerSQLTestValue(rawMaxRows)
		if !ok || value < 1 || value > maxSQLTestRows {
			return uuid.Nil, "", "", 0, fmt.Errorf("tool: SQL max_rows must be an integer between 1 and %d", maxSQLTestRows)
		}
		maxRows = value
	}

	return datasourceID, query, operation, maxRows, nil
}

func sameSQLDatasourceID(left, right any) bool {
	leftString, leftOK := left.(string)
	rightString, rightOK := right.(string)
	if !leftOK || !rightOK {
		return false
	}
	leftID, leftErr := uuid.Parse(strings.TrimSpace(leftString))
	rightID, rightErr := uuid.Parse(strings.TrimSpace(rightString))
	if leftErr == nil && rightErr == nil {
		return leftID == rightID
	}
	return strings.TrimSpace(leftString) == strings.TrimSpace(rightString)
}

func integerSQLTestValue(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, true
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (postgresSQLTestExecutor) Execute(ctx context.Context, credentials DatasourceCreds, query string, inputs map[string]any, operation string, maxRows int) (string, error) {
	conn, err := connectPostgresSQLTest(ctx, credentials)
	if err != nil {
		return "", fmt.Errorf("tool: SQL connection failed")
	}
	defer func() { _ = conn.Close(ctx) }()

	renderedQuery, args, err := renderSQLTestQuery(query, inputs)
	if err != nil {
		return "", fmt.Errorf("tool: SQL query inputs are invalid: %w", err)
	}

	if operation == "SELECT" {
		rows, err := conn.Query(ctx, renderedQuery, args...)
		if err != nil {
			return "", fmt.Errorf("tool: SQL query failed")
		}
		defer rows.Close()
		result, err := collectSQLTestRows(rows, maxRows)
		if err != nil {
			return "", fmt.Errorf("tool: SQL query failed")
		}
		return marshalSQLTestResult(map[string]any{"rows": result})
	}

	tag, err := conn.Exec(ctx, renderedQuery, args...)
	if err != nil {
		return "", fmt.Errorf("tool: SQL query failed")
	}
	return marshalSQLTestResult(map[string]any{"rowsAffected": tag.RowsAffected()})
}

func connectPostgresSQLTest(ctx context.Context, credentials DatasourceCreds) (*pgx.Conn, error) {
	dsn := (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(credentials.User, credentials.Password),
		Host:   net.JoinHostPort(credentials.Host, strconv.Itoa(credentials.Port)),
		Path:   credentials.Database,
	}).String()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	resolvedHost, err := resolveSQLTestHost(ctx, config.Host)
	if err != nil {
		return nil, err
	}
	config.Host = resolvedHost
	for _, fallback := range config.Fallbacks {
		fallback.Host = resolvedHost
	}
	dialer := &net.Dialer{}
	config.DialFunc = dialer.DialContext
	return pgx.ConnectConfig(ctx, config)
}

func resolveSQLTestHost(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if blockedSQLTestIP(ip) {
			return "", fmt.Errorf("blocked datasource address")
		}
		return ip.String(), nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return "", fmt.Errorf("resolve datasource host")
	}
	for _, address := range addresses {
		if blockedSQLTestIP(address.IP) {
			return "", fmt.Errorf("blocked datasource address")
		}
	}
	return addresses[0].IP.String(), nil
}

func blockedSQLTestIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func collectSQLTestRows(rows pgx.Rows, maxRows int) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	result := make([]map[string]any, 0)
	for rows.Next() {
		if len(result) >= maxRows {
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(fields))
		for i, field := range fields {
			row[string(field.Name)] = values[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func marshalSQLTestResult(value any) (string, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("tool: encode SQL result")
	}
	return string(encoded), nil
}

var sqlTestInputPlaceholderRE = regexp.MustCompile(`'?\{\{input\.([A-Za-z0-9_]+)\}\}'?`)
var sqlTestParamRE = regexp.MustCompile(`\$([1-9][0-9]*)`)

func renderSQLTestQuery(template string, inputs map[string]any) (string, []any, error) {
	positions := make(map[int]bool)
	maxPosition := 0
	for _, match := range sqlTestParamRE.FindAllStringSubmatch(template, -1) {
		position, _ := strconv.Atoi(match[1])
		positions[position] = true
		if position > maxPosition {
			maxPosition = position
		}
	}
	args := make([]any, 0, maxPosition)
	for position := 1; position <= maxPosition; position++ {
		if !positions[position] {
			return "", nil, fmt.Errorf("SQL parameters must be contiguous starting at $1; missing $%d", position)
		}
		value, ok := lookupSQLTestInput(inputs, position)
		if !ok {
			return "", nil, fmt.Errorf("missing value for SQL parameter $%d", position)
		}
		args = append(args, value)
	}

	nextPosition := maxPosition
	rendered := sqlTestInputPlaceholderRE.ReplaceAllStringFunc(template, func(match string) string {
		parts := sqlTestInputPlaceholderRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		nextPosition++
		args = append(args, inputs[parts[1]])
		return "$" + strconv.Itoa(nextPosition)
	})
	return rendered, args, nil
}

func lookupSQLTestInput(inputs map[string]any, position int) (any, bool) {
	if inputs == nil {
		return nil, false
	}
	keys := []string{"$" + strconv.Itoa(position), strconv.Itoa(position), "arg" + strconv.Itoa(position), "param" + strconv.Itoa(position)}
	for _, key := range keys {
		if value, ok := inputs[key]; ok {
			return value, true
		}
	}
	for _, containerKey := range []string{"parameters", "args"} {
		container, ok := inputs[containerKey]
		if !ok {
			continue
		}
		switch value := container.(type) {
		case []any:
			if position <= len(value) {
				return value[position-1], true
			}
		case map[string]any:
			for _, key := range keys {
				if nested, ok := value[key]; ok {
					return nested, true
				}
			}
		}
	}
	return nil, false
}
