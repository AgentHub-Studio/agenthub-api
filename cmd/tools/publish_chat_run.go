package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ChatRunTask struct {
	RunID     string `json:"runId"`
	SessionID string `json:"sessionId"`
	TenantID  string `json:"tenantId"`
	Message   string `json:"message"`
}

func main() {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		log.Fatal("RABBITMQ_URL is required")
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	q, err := ch.QueueDeclare("chat.run.queue", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	task := ChatRunTask{
		RunID:     os.Getenv("RUN_ID"),
		SessionID: os.Getenv("SESSION_ID"),
		TenantID:  os.Getenv("SCHEMA"),
		Message:   os.Getenv("MESSAGE"),
	}

	if task.RunID == "" || task.SessionID == "" || task.TenantID == "" || task.Message == "" {
		log.Fatalf("RUN_ID, SESSION_ID, MESSAGE and SCHEMA are required")
	}

	body, _ := json.Marshal(task)

	err = ch.PublishWithContext(context.Background(), "", q.Name, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		log.Fatalf("Failed to publish a message: %v", err)
	}

	log.Print(chatRunPublishedLogMessage())
}

func chatRunPublishedLogMessage() string {
	return " [x] Sent message to chat.run.queue"
}
