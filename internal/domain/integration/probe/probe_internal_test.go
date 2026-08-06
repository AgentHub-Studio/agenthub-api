package probe

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
)

type deadlineProbeError struct{}

func (deadlineProbeError) Error() string { return "probe deadline" }

func (deadlineProbeError) Is(target error) bool { return target == context.DeadlineExceeded }

func TestMapPostgresError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		hint string
	}{
		{name: "invalid credentials", err: errors.New("password authentication failed"), hint: "Usuário ou senha inválidos"},
		{name: "database missing", err: errors.New("database does not exist"), hint: "Banco ou schema não encontrado"},
		{name: "unknown host", err: errors.New("no such host"), hint: "Host não encontrado"},
		{name: "connection refused", err: errors.New("connection refused"), hint: "Conexão recusada"},
		{name: "timeout", err: errors.New("i/o timeout"), hint: "Tempo esgotado — o banco"},
		{name: "context deadline", err: deadlineProbeError{}, hint: "Tempo esgotado"},
		{name: "unknown error", err: errors.New("unexpected database failure"), hint: "unexpected database failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, mapPostgresError(tt.err), tt.hint)
		})
	}
}

func TestDatabase_PostgresConnectionFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	got := NewService().Database(context.Background(), DatabaseRequest{
		Type:     datasource.DataSourceTypePostgreSQL,
		Host:     "127.0.0.1",
		Port:     port,
		Database: "agenthub",
		DBUser:   "agenthub",
	})

	assert.False(t, got.OK)
	assert.NotEmpty(t, got.ErrorHint)
}

func TestTruncateAddsEllipsisWhenLimitIsExceeded(t *testing.T) {
	assert.Equal(t, "abc…", truncate("abcd", 3))
}

func TestProbeSecretRedactorAddIgnoresBlankAndDuplicateValues(t *testing.T) {
	var redactor probeSecretRedactor
	redactor.add("  ")
	redactor.add("secret")
	redactor.add("secret")

	assert.Equal(t, []string{"secret"}, redactor.values)
}
