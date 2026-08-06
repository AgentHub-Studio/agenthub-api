package a2a

type CreateGrantRequest struct {
	SubjectTenant string   `json:"subjectTenant"`
	AgentID       string   `json:"agentId"`
	Actions       []string `json:"actions"`
}

type GrantResponse struct {
	ID            string   `json:"id"`
	SubjectTenant string   `json:"subjectTenant"`
	AgentID       string   `json:"agentId"`
	Actions       []string `json:"actions"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

type InvokeRequest struct {
	TargetTenant string `json:"targetTenant"`
	AgentID      string `json:"agentId"`
	Input        string `json:"input"`
}

func GrantResponseFrom(g Grant) GrantResponse {
	return GrantResponse{
		ID:            g.ID.String(),
		SubjectTenant: g.SubjectTenant,
		AgentID:       g.AgentID.String(),
		Actions:       append([]string{}, g.Actions...),
		CreatedAt:     g.CreatedAt.Format(timeLayout),
		UpdatedAt:     g.UpdatedAt.Format(timeLayout),
	}
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
