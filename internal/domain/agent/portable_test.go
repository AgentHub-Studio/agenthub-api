package agent_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
)

func TestPortableRoundTrip(t *testing.T) {
	prompt := "You are a helpful assistant."
	skillID := uuid.New()
	kbID := uuid.New()
	a := agent.Agent{
		ID:              uuid.New(),
		Name:            "Support Bot",
		Slug:            "support-bot",
		Description:     "Handles tier-1 tickets.",
		Status:          agent.StatusPublished,
		SystemPrompt:    &prompt,
		ModelConfig:     json.RawMessage(`{"provider":"anthropic","model":"claude-opus-4-6","apiKey":"secret-must-be-stripped"}`),
		PermissionRules: json.RawMessage(`{"mode":"default_allow"}`),
		Config:          json.RawMessage(`{}`),
	}

	p := agent.ToPortable(a, []uuid.UUID{skillID}, []uuid.UUID{kbID})
	if p.APIVersion != agent.PortableAPIVersion || p.Kind != agent.PortableKind {
		t.Fatalf("unexpected envelope: %+v", p)
	}
	if p.Metadata.Slug != "support-bot" || p.Metadata.Name != "Support Bot" {
		t.Fatalf("metadata mismatch: %+v", p.Metadata)
	}
	if p.Spec.Instructions != prompt {
		t.Fatalf("instructions lost: %q", p.Spec.Instructions)
	}
	if strings.Contains(string(p.Spec.ModelConfig), "apiKey") {
		t.Fatalf("apiKey leaked in exported modelConfig: %s", p.Spec.ModelConfig)
	}
	if len(p.Spec.SkillIDs) != 1 || p.Spec.SkillIDs[0] != skillID {
		t.Fatalf("skillIds mismatch")
	}
	if len(p.Spec.KnowledgeBaseIDs) != 1 || p.Spec.KnowledgeBaseIDs[0] != kbID {
		t.Fatalf("knowledgeBaseIds mismatch")
	}

	data, err := agent.MarshalPortableYAML(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "apiVersion: agenthub.io/v1") {
		t.Fatalf("yaml missing apiVersion: %s", data)
	}

	parsed, err := agent.UnmarshalPortableYAML(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	req := parsed.ToCreateRequest()
	if req.Name != a.Name || req.Slug != a.Slug {
		t.Fatalf("round-trip identity lost: %+v", req)
	}
	if req.SystemPrompt == nil || *req.SystemPrompt != prompt {
		t.Fatalf("systemPrompt lost")
	}
	if len(req.SkillIDs) != 1 || req.SkillIDs[0] != skillID {
		t.Fatalf("skillIds round-trip lost")
	}
}

func TestUnmarshalRejectsWrongAPIVersion(t *testing.T) {
	data := []byte("apiVersion: wrong/v2\nkind: Agent\nmetadata:\n  name: x\n  slug: x\n")
	if _, err := agent.UnmarshalPortableYAML(data); err == nil {
		t.Fatalf("expected error for wrong apiVersion")
	}
}

func TestUnmarshalRequiresNameAndSlug(t *testing.T) {
	data := []byte("apiVersion: agenthub.io/v1\nkind: Agent\nmetadata: {}\n")
	if _, err := agent.UnmarshalPortableYAML(data); err == nil {
		t.Fatalf("expected error for missing name/slug")
	}
}
