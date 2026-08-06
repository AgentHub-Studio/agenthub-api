package channel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
)

func TestPlatformAdaptersParseMessage_RejectDuplicateJSONKeys(t *testing.T) {
	tests := []struct {
		name    string
		adapter channel.Adapter
		body    string
	}{
		{
			name:    "slack",
			adapter: &channel.SlackAdapter{},
			body:    `{"type":"url_verification","\u0074ype":"event_callback"}`,
		},
		{
			name:    "telegram",
			adapter: &channel.TelegramAdapter{},
			body:    `{"update_id":1,"\u0075pdate_id":2}`,
		},
		{
			name:    "discord",
			adapter: &channel.DiscordAdapter{},
			body:    `{"type":1,"\u0074ype":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := tt.adapter.ParseMessage(context.Background(), channel.Channel{}, []byte(tt.body))

			require.Error(t, err)
			assert.Empty(t, message)
		})
	}
}
