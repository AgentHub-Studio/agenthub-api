//go:build integration

package chat

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type workerRecoveryRunner struct {
	calls   atomic.Int32
	started chan struct{}
}

func (r *workerRecoveryRunner) RunSession(context.Context, RunInput) (<-chan RunEvent, error) {
	r.calls.Add(1)
	r.started <- struct{}{}
	events := make(chan RunEvent, 1)
	events <- RunEvent{Type: "run_complete"}
	close(events)
	return events, nil
}

func startRabbitMQForWorkerRecovery(t *testing.T) (testcontainers.Container, string) {
	t.Helper()
	ctx := context.Background()
	broker, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "rabbitmq:3.13-alpine",
			ExposedPorts: []string{"5672/tcp"},
			WaitingFor:   wait.ForListeningPort("5672/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Terminate(ctx) })

	host, err := broker.Host(ctx)
	require.NoError(t, err)
	port, err := broker.MappedPort(ctx, "5672/tcp")
	require.NoError(t, err)
	return broker, fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())
}

func waitForWorkerRun(t *testing.T, runner *workerRecoveryRunner) {
	t.Helper()
	select {
	case <-runner.started:
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not process the queued run")
	}
}

func deleteChatRunQueue(t *testing.T, brokerURL string) {
	t.Helper()
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	channel, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = channel.Close() }()
	_, err = channel.QueueDelete(ChatRunQueue, false, false, false)
	require.NoError(t, err)
}

func waitForRabbitMQ(t *testing.T, brokerURL string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := amqp.Dial(brokerURL)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("RabbitMQ did not become reachable after restart")
}

func runRabbitMQCTL(t *testing.T, broker testcontainers.Container, command string) {
	t.Helper()
	exitCode, output, err := broker.Exec(context.Background(), []string{"rabbitmqctl", command})
	require.NoError(t, err)
	body, readErr := io.ReadAll(output)
	require.NoError(t, readErr)
	require.Equalf(t, 0, exitCode, "rabbitmqctl %s failed: %s", command, body)
}

func TestIntegration_AsyncWorkerRecoversAfterRabbitMQQueueDeletion(t *testing.T) {
	_, brokerURL := startRabbitMQForWorkerRecovery(t)
	runner := &workerRecoveryRunner{started: make(chan struct{}, 2)}
	agentID := uuid.New()
	sessionID := uuid.New()
	repo := &asyncExecutorRepoStub{session: ChatSession{ID: sessionID, AgentID: &agentID}}
	executor := NewAsyncExecutor(repo, runner, brokerURL)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	workerResult := make(chan error, 1)
	go func() { workerResult <- executor.StartWorker(workerCtx) }()

	_, err := executor.EnqueueRun(context.Background(), sessionID, "test", "before restart")
	require.NoError(t, err)
	waitForWorkerRun(t, runner)

	deleteChatRunQueue(t, brokerURL)

	select {
	case err := <-workerResult:
		t.Fatalf("worker stopped instead of waiting for broker recovery: %v", err)
	case <-time.After(5 * time.Second):
	}

	_, err = executor.EnqueueRun(context.Background(), sessionID, "test", "after restart")
	require.NoError(t, err)
	waitForWorkerRun(t, runner)
	require.Equal(t, int32(2), runner.calls.Load())

	cancelWorker()
	select {
	case err := <-workerResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestIntegration_AsyncWorkerRecoversAfterRabbitMQApplicationRestart(t *testing.T) {
	broker, brokerURL := startRabbitMQForWorkerRecovery(t)
	runner := &workerRecoveryRunner{started: make(chan struct{}, 2)}
	agentID := uuid.New()
	sessionID := uuid.New()
	repo := &asyncExecutorRepoStub{session: ChatSession{ID: sessionID, AgentID: &agentID}}
	executor := NewAsyncExecutor(repo, runner, brokerURL)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	workerResult := make(chan error, 1)
	go func() { workerResult <- executor.StartWorker(workerCtx) }()

	_, err := executor.EnqueueRun(context.Background(), sessionID, "test", "before broker restart")
	require.NoError(t, err)
	waitForWorkerRun(t, runner)

	runRabbitMQCTL(t, broker, "stop_app")
	select {
	case err := <-workerResult:
		t.Fatalf("worker stopped instead of reconnecting after broker restart: %v", err)
	case <-time.After(2 * time.Second):
	}

	runRabbitMQCTL(t, broker, "start_app")
	waitForRabbitMQ(t, brokerURL)

	_, err = executor.EnqueueRun(context.Background(), sessionID, "test", "after broker restart")
	require.NoError(t, err)
	waitForWorkerRun(t, runner)
	require.Equal(t, int32(2), runner.calls.Load())

	cancelWorker()
	select {
	case err := <-workerResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}
