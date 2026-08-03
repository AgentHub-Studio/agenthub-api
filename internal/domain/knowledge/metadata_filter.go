package knowledge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/metadata"
)

// MetadataFilter is a validated expression over the metadata of the document
// that owns a search chunk. It is intentionally opaque: callers construct it
// with ParseMetadataFilter so every SQL predicate stays parameterized.
type MetadataFilter struct {
	path     []string
	op       string
	value    json.RawMessage
	values   []json.RawMessage
	strings  []string
	pattern  string
	number   string
	children []MetadataFilter
	child    *MetadataFilter
}

const (
	metadataOpEq          = "eq"
	metadataOpNeq         = "neq"
	metadataOpIn          = "in"
	metadataOpNotIn       = "notIn"
	metadataOpExists      = "exists"
	metadataOpNotExists   = "notExists"
	metadataOpGt          = "gt"
	metadataOpGte         = "gte"
	metadataOpLt          = "lt"
	metadataOpLte         = "lte"
	metadataOpLike        = "like"
	metadataOpILike       = "ilike"
	metadataOpContainsAny = "containsAny"
	metadataOpContainsAll = "containsAll"

	// MaxMetadataFilterBytes bounds one encoded filter before recursive JSON
	// validation and SQL predicate construction. It matches document metadata.
	MaxMetadataFilterBytes = 16 << 10

	maxMetadataFilterDepth      = 8
	maxMetadataFilterChildren   = 32
	maxMetadataFilterPredicates = 64
	maxMetadataFilterValues     = 64
)

type metadataFilterParseState struct {
	predicates int
}

// ParseMetadataFilter validates the V1 filter grammar. A missing field is no
// filter; a present value must be one predicate or one boolean group.
func ParseMetadataFilter(raw json.RawMessage) (*MetadataFilter, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > MaxMetadataFilterBytes {
		return nil, fmt.Errorf("metadata filter exceeds maximum size of %d bytes", MaxMetadataFilterBytes)
	}
	validated, err := httputil.DecodeSingleRawJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("metadata filter must contain one JSON value with unique object keys: %w", err)
	}

	object, err := decodeObject(validated)
	if err != nil {
		return nil, fmt.Errorf("metadata filter must be an object: %w", err)
	}
	return parseMetadataExpression(object, 1, &metadataFilterParseState{})
}

func parseMetadataExpression(object map[string]json.RawMessage, depth int, state *metadataFilterParseState) (*MetadataFilter, error) {
	if depth > maxMetadataFilterDepth {
		return nil, fmt.Errorf("metadata filter exceeds maximum nesting depth of %d", maxMetadataFilterDepth)
	}
	if len(object) == 0 {
		return nil, fmt.Errorf("metadata filter must not be empty")
	}

	if raw, ok := object["all"]; ok {
		if len(object) != 1 {
			return nil, fmt.Errorf("metadata filter group must contain exactly one operator")
		}
		children, err := parseMetadataChildren(raw, depth, state)
		if err != nil {
			return nil, fmt.Errorf("all: %w", err)
		}
		return &MetadataFilter{children: children, op: "all"}, nil
	}
	if raw, ok := object["any"]; ok {
		if len(object) != 1 {
			return nil, fmt.Errorf("metadata filter group must contain exactly one operator")
		}
		children, err := parseMetadataChildren(raw, depth, state)
		if err != nil {
			return nil, fmt.Errorf("any: %w", err)
		}
		return &MetadataFilter{children: children, op: "any"}, nil
	}
	if raw, ok := object["not"]; ok {
		if len(object) != 1 {
			return nil, fmt.Errorf("metadata filter group must contain exactly one operator")
		}
		childObject, err := decodeObject(raw)
		if err != nil {
			return nil, fmt.Errorf("not must contain an object: %w", err)
		}
		child, err := parseMetadataExpression(childObject, depth+1, state)
		if err != nil {
			return nil, fmt.Errorf("not: %w", err)
		}
		return &MetadataFilter{child: child, op: "not"}, nil
	}

	return parseMetadataPredicate(object, state)
}

func parseMetadataChildren(raw json.RawMessage, depth int, state *metadataFilterParseState) ([]MetadataFilter, error) {
	if !isJSONArray(raw) {
		return nil, fmt.Errorf("must contain an array of expressions")
	}
	var rawChildren []json.RawMessage
	if err := decode(raw, &rawChildren); err != nil {
		return nil, fmt.Errorf("must contain an array of expressions")
	}
	if len(rawChildren) == 0 {
		return nil, fmt.Errorf("must not be empty")
	}
	if len(rawChildren) > maxMetadataFilterChildren {
		return nil, fmt.Errorf("must contain at most %d expressions", maxMetadataFilterChildren)
	}

	children := make([]MetadataFilter, 0, len(rawChildren))
	for _, rawChild := range rawChildren {
		object, err := decodeObject(rawChild)
		if err != nil {
			return nil, fmt.Errorf("child must be an object: %w", err)
		}
		child, err := parseMetadataExpression(object, depth+1, state)
		if err != nil {
			return nil, err
		}
		children = append(children, *child)
	}
	return children, nil
}

func parseMetadataPredicate(object map[string]json.RawMessage, state *metadataFilterParseState) (*MetadataFilter, error) {
	if len(object) < 2 || len(object) > 3 {
		return nil, fmt.Errorf("metadata predicate must contain field, op, and an optional value")
	}
	rawField, hasField := object["field"]
	rawOp, hasOp := object["op"]
	if !hasField || !hasOp {
		return nil, fmt.Errorf("metadata predicate requires field and op")
	}
	state.predicates++
	if state.predicates > maxMetadataFilterPredicates {
		return nil, fmt.Errorf("metadata filter contains more than %d predicates", maxMetadataFilterPredicates)
	}
	for key := range object {
		if key != "field" && key != "op" && key != "value" {
			return nil, fmt.Errorf("metadata predicate contains unsupported key %q", key)
		}
	}

	var field, op string
	if err := decode(rawField, &field); err != nil {
		return nil, fmt.Errorf("metadata predicate field must be a non-empty string")
	}
	path, err := metadata.ParseFieldPath(field)
	if err != nil {
		return nil, fmt.Errorf("metadata predicate field %w", err)
	}
	if err := decode(rawOp, &op); err != nil {
		return nil, fmt.Errorf("metadata predicate op must be a string")
	}

	rawValue, hasValue := object["value"]
	if op == metadataOpExists || op == metadataOpNotExists {
		if hasValue {
			return nil, fmt.Errorf("%s does not accept value", op)
		}
		return &MetadataFilter{path: path, op: op}, nil
	}
	if !hasValue {
		return nil, fmt.Errorf("metadata predicate %q requires value", op)
	}

	filter := &MetadataFilter{path: path, op: op}
	switch op {
	case metadataOpEq, metadataOpNeq:
		if err := validateMetadataValue(rawValue, len(path) == 1); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		filter.value = rawValue
	case metadataOpIn, metadataOpNotIn:
		values, err := parseScalarValues(rawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		filter.values = values
	case metadataOpGt, metadataOpGte, metadataOpLt, metadataOpLte:
		number, err := parseNumber(rawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		filter.number = number
	case metadataOpLike, metadataOpILike:
		pattern, err := parseString(rawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		filter.pattern = pattern
	case metadataOpContainsAny, metadataOpContainsAll:
		values, err := parseStrings(rawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		filter.strings = values
	default:
		return nil, fmt.Errorf("unsupported metadata operator %q", op)
	}

	return filter, nil
}

func validateMetadataValue(raw json.RawMessage, allowObject bool) error {
	var value any
	if err := decode(raw, &value); err != nil {
		return fmt.Errorf("invalid JSON value: %w", err)
	}
	if !allowObject {
		if _, ok := value.(map[string]any); ok {
			return fmt.Errorf("nested field values must be strings, numbers, booleans, or string arrays")
		}
	}
	if values, ok := value.([]any); ok && len(values) > maxMetadataFilterValues {
		return fmt.Errorf("value must contain at most %d items", maxMetadataFilterValues)
	}
	return metadata.ValidateValue(value)
}

func parseNumber(raw json.RawMessage) (string, error) {
	var value any
	if err := decode(raw, &value); err != nil {
		return "", fmt.Errorf("value must be a number: %w", err)
	}
	number, ok := value.(json.Number)
	if !ok {
		return "", fmt.Errorf("value must be a number")
	}
	if err := metadata.ValidateValue(number); err != nil {
		return "", err
	}
	return number.String(), nil
}

func parseString(raw json.RawMessage) (string, error) {
	var value any
	if err := decode(raw, &value); err != nil {
		return "", fmt.Errorf("value must be a string: %w", err)
	}
	stringValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be a string")
	}
	if err := metadata.ValidateValue(stringValue); err != nil {
		return "", err
	}
	return stringValue, nil
}

func parseScalarValues(raw json.RawMessage) ([]json.RawMessage, error) {
	if !isJSONArray(raw) {
		return nil, fmt.Errorf("value must be an array")
	}
	var values []json.RawMessage
	if err := decode(raw, &values); err != nil {
		return nil, fmt.Errorf("value must be an array")
	}
	if len(values) > maxMetadataFilterValues {
		return nil, fmt.Errorf("value must contain at most %d items", maxMetadataFilterValues)
	}
	for _, value := range values {
		var decoded any
		if err := decode(value, &decoded); err != nil {
			return nil, fmt.Errorf("invalid list value: %w", err)
		}
		if !metadata.IsScalar(decoded) {
			return nil, fmt.Errorf("values must be strings, numbers, or booleans")
		}
		if err := metadata.ValidateValue(decoded); err != nil {
			return nil, err
		}
	}
	return values, nil
}

func parseStrings(raw json.RawMessage) ([]string, error) {
	if !isJSONArray(raw) {
		return nil, fmt.Errorf("value must be an array of strings")
	}
	var rawValues []json.RawMessage
	if err := decode(raw, &rawValues); err != nil {
		return nil, fmt.Errorf("value must be an array of strings: %w", err)
	}
	if len(rawValues) > maxMetadataFilterValues {
		return nil, fmt.Errorf("value must contain at most %d items", maxMetadataFilterValues)
	}
	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		var value any
		if err := decode(rawValue, &value); err != nil {
			return nil, fmt.Errorf("invalid array value: %w", err)
		}
		stringValue, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("value must contain only strings")
		}
		if err := metadata.ValidateValue(stringValue); err != nil {
			return nil, err
		}
		values = append(values, stringValue)
	}
	return values, nil
}

func isJSONArray(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '['
}

func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := decode(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("must be an object")
	}
	return object, nil
}

func decode(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("unexpected additional JSON value")
	}
	return err
}

// SQL returns a parameterized SQL predicate that references document metadata
// through the d table alias. The first generated placeholder is start+1.
func (f *MetadataFilter) SQL(start int) (string, []any) {
	if f == nil {
		return "", nil
	}
	builder := metadataSQLBuilder{next: start}
	return builder.expression(f), builder.args
}

type metadataSQLBuilder struct {
	next int
	args []any
}

func (b *metadataSQLBuilder) parameter(value any) string {
	b.next++
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", b.next)
}

func (b *metadataSQLBuilder) expression(filter *MetadataFilter) string {
	switch filter.op {
	case "all", "any":
		parts := make([]string, 0, len(filter.children))
		for i := range filter.children {
			parts = append(parts, b.expression(&filter.children[i]))
		}
		joiner := " AND "
		if filter.op == "any" {
			joiner = " OR "
		}
		return "(" + strings.Join(parts, joiner) + ")"
	case "not":
		return "(NOT " + b.expression(filter.child) + ")"
	case metadataOpEq, metadataOpNeq:
		path := b.parameter(filter.path)
		value := b.parameter(string(filter.value))
		operator := "="
		if filter.op == metadataOpNeq {
			operator = "<>"
		}
		return fmt.Sprintf("((d.metadata #> %s::text[]) %s %s::jsonb)", path, operator, value)
	case metadataOpIn, metadataOpNotIn:
		if len(filter.values) == 0 {
			if filter.op == metadataOpNotIn {
				return "(TRUE)"
			}
			return "(FALSE)"
		}
		path := b.parameter(filter.path)
		parts := make([]string, 0, len(filter.values))
		for _, value := range filter.values {
			parts = append(parts, b.parameter(string(value))+"::jsonb")
		}
		operator := "IN"
		if filter.op == metadataOpNotIn {
			operator = "NOT IN"
		}
		return fmt.Sprintf("((d.metadata #> %s::text[]) %s (%s))", path, operator, strings.Join(parts, ", "))
	case metadataOpExists:
		path := b.parameter(filter.path)
		return fmt.Sprintf("((d.metadata #> %s::text[]) IS NOT NULL)", path)
	case metadataOpNotExists:
		path := b.parameter(filter.path)
		return fmt.Sprintf("((d.metadata #> %s::text[]) IS NULL)", path)
	case metadataOpGt, metadataOpGte, metadataOpLt, metadataOpLte:
		path := b.parameter(filter.path)
		value := b.parameter(filter.number)
		operator := map[string]string{
			metadataOpGt:  ">",
			metadataOpGte: ">=",
			metadataOpLt:  "<",
			metadataOpLte: "<=",
		}[filter.op]
		return fmt.Sprintf("((jsonb_typeof(d.metadata #> %s::text[]) = 'number') AND ((d.metadata #>> %s::text[])::numeric %s %s::numeric))", path, path, operator, value)
	case metadataOpLike, metadataOpILike:
		path := b.parameter(filter.path)
		pattern := b.parameter(filter.pattern)
		operator := "LIKE"
		if filter.op == metadataOpILike {
			operator = "ILIKE"
		}
		return fmt.Sprintf("((jsonb_typeof(d.metadata #> %s::text[]) = 'string') AND ((d.metadata #>> %s::text[]) %s %s))", path, path, operator, pattern)
	case metadataOpContainsAny:
		if len(filter.strings) == 0 {
			return "(FALSE)"
		}
		path := b.parameter(filter.path)
		values := b.parameter(filter.strings)
		return fmt.Sprintf("((d.metadata #> %s::text[]) ?| %s::text[])", path, values)
	case metadataOpContainsAll:
		if len(filter.strings) == 0 {
			return "(TRUE)"
		}
		path := b.parameter(filter.path)
		values := b.parameter(filter.strings)
		return fmt.Sprintf("((d.metadata #> %s::text[]) ?& %s::text[])", path, values)
	default:
		return "(FALSE)"
	}
}
