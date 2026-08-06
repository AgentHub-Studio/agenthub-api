package channel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
)

func TestCustomAdapterParseMessage_RejectsDuplicateJSONKeys(t *testing.T) {
	adapter := &channel.CustomAdapter{}

	for _, body := range []string{
		`{"text":"first","text":"second"}`,
		`{"text":"first","\u0074ext":"second"}`,
	} {
		message, err := adapter.ParseMessage(context.Background(), channel.Channel{}, []byte(body))

		require.Error(t, err, body)
		assert.Empty(t, message.Text, body)
	}
}
