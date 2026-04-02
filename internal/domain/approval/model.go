// Package approval manages pipeline execution approval requests.
package approval

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status values for a pending approval.
const (
	StatusPending  = "PENDING"
	StatusApproved = "APPROVED"
	StatusRejected = "REJECTED"
	StatusTimeout  = "TIMEOUT"
)

var (
	ErrNotFound        = errors.New("approval: not found")
	ErrAlreadyResolved = errors.New("approval: already resolved")
)

// PendingApproval represents a human-in-the-loop approval request created by an APPROVAL node.
type PendingApproval struct {
	ID             uuid.UUID  `json:"id"`
	ExecutionID    uuid.UUID  `json:"executionId"`
	NodeID         string     `json:"nodeId"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Details        string     `json:"details"`
	Status         string     `json:"status"`
	RespondedBy    *string    `json:"respondedBy,omitempty"`
	RespondedAt    *time.Time `json:"respondedAt,omitempty"`
	Comment        *string    `json:"comment,omitempty"`
	TimeoutAt      *time.Time `json:"timeoutAt,omitempty"`
	CallbackURL    *string    `json:"callbackUrl,omitempty"`
	NotifyChannels []string   `json:"notifyChannels"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// CreateApprovalRequest is the payload to create an approval request.
type CreateApprovalRequest struct {
	ExecutionID    uuid.UUID  `json:"executionId"    validate:"required"`
	NodeID         string     `json:"nodeId"         validate:"required"`
	Title          string     `json:"title"          validate:"required"`
	Description    string     `json:"description"`
	Details        string     `json:"details"`
	TimeoutAt      *time.Time `json:"timeoutAt"`
	CallbackURL    *string    `json:"callbackUrl"`
	NotifyChannels []string   `json:"notifyChannels"`
}

// RespondRequest is the payload to approve or reject a pending approval.
type RespondRequest struct {
	Approved bool    `json:"approved"  validate:"required"`
	Comment  *string `json:"comment"`
}

// PendingCountResponse is returned by GET /api/approvals/pending-count.
type PendingCountResponse struct {
	Count int64 `json:"count"`
}
