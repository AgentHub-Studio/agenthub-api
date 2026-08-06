package workflow

// CreateRequest is the payload for POST /api/workflows.
type CreateRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Slug        string  `json:"slug,omitempty"`
	Description *string `json:"description,omitempty"`
	Start       string  `json:"start,omitempty"`
	Steps       []Step  `json:"steps" validate:"required"`
}

// ExecuteRequest starts a workflow execution.
type ExecuteRequest struct {
	Input map[string]any `json:"input,omitempty"`
}

// ResumeRequest resumes a suspended workflow execution.
type ResumeRequest map[string]any
