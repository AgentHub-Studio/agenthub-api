package probe

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
)

func TestPostgresProbeDSNEscapesReservedCredentials(t *testing.T) {
	req := DatabaseRequest{
		Type:       datasource.DataSourceTypePostgreSQL,
		Host:       "postgres",
		Port:       5432,
		Database:   "agenthub-db/probe",
		DBUser:     "probe/user",
		DBPassword: "p@ss/word#frag?x=1",
	}

	config, err := pgx.ParseConfig(postgresProbeDSN(req))

	require.NoError(t, err)
	assert.Equal(t, req.Host, config.Host)
	assert.Equal(t, req.Database, config.Database)
	assert.Equal(t, req.DBUser, config.User)
	assert.Equal(t, req.DBPassword, config.Password)
}
