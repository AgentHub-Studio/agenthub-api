// Package webhook manages webhook configurations and delivery logs.
package webhook

import (
	"time"

	"github.com/google/uuid"
)

// WebhookConfig represents a webhook endpoint configuration.
type WebhookConfig struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Events     []string  `json:"events"`
	Secret     *string   `json:"secret,omitempty"`
	Token      string    `json:"token"`  // auto-generated authentication token for ingestion
	Enabled    bool      `json:"enabled"`
	RetryCount int       `json:"retryCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Delivery status constants.
const (
	DeliveryPending    = "PENDING"
	DeliveryProcessing = "PROCESSING"
	DeliverySuccess    = "SUCCESS"
	DeliveryFailed     = "FAILED"
	DeliveryDeadLetter = "DEAD_LETTER"
)

// DeliveryFilter holds optional filters for listing delivery logs.
type DeliveryFilter struct {
	Status    string // optional: PENDING, PROCESSING, SUCCESS, FAILED, DEAD_LETTER
	EventType string // optional: event type string
}

// WebhookDeliveryLog records a delivery attempt.
type WebhookDeliveryLog struct {
	ID             uuid.UUID  `json:"id"`
	WebhookID      uuid.UUID  `json:"webhookId"`
	EventType      string     `json:"eventType"`
	Payload        []byte     `json:"payload"`
	Status         string     `json:"status"` // PENDING, DELIVERED, FAILED
	ResponseStatus *int       `json:"responseStatus,omitempty"`
	ResponseBody   *string    `json:"responseBody,omitempty"`
	Attempts       int        `json:"attempts"`
	DeliveredAt    *time.Time `json:"deliveredAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}
