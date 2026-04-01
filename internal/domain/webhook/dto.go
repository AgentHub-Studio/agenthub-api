package webhook

// CreateWebhookRequest is the request body for POST /api/webhooks.
type CreateWebhookRequest struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Events     []string `json:"events"`
	Secret     *string  `json:"secret,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`
	RetryCount *int     `json:"retryCount,omitempty"`
}

// UpdateWebhookRequest is the request body for PUT /api/webhooks/{id}.
type UpdateWebhookRequest struct {
	Name       *string  `json:"name,omitempty"`
	URL        *string  `json:"url,omitempty"`
	Events     []string `json:"events,omitempty"`
	Secret     *string  `json:"secret,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`
	RetryCount *int     `json:"retryCount,omitempty"`
}
