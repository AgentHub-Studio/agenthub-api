package oauth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
)

func TestAuthTypeOAuth2ClientCredentialsValue(t *testing.T) {
	assert.Equal(t, oauth.AuthType("OAUTH2_CLIENT_CREDENTIALS"), oauth.AuthTypeOAuth2ClientCredentials)
}
