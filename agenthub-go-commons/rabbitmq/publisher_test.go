package rabbitmq_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/rabbitmq"
)

func TestNewPublisher_InvalidURL(t *testing.T) {
	_, err := rabbitmq.NewPublisher("amqp://invalid-host-that-does-not-exist:5672/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rabbitmq: dial")
}

func TestNewPublisher_MalformedURL(t *testing.T) {
	_, err := rabbitmq.NewPublisher("not-a-valid-amqp-url")
	require.Error(t, err)
}
