package rabbitmq_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/rabbitmq"
)

func TestNewConsumer_InvalidURL(t *testing.T) {
	_, err := rabbitmq.NewConsumer("amqp://invalid-host-that-does-not-exist:5672/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rabbitmq: consumer dial")
}

func TestNewConsumer_MalformedURL(t *testing.T) {
	_, err := rabbitmq.NewConsumer("not-a-valid-amqp-url")
	require.Error(t, err)
}
