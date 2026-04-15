package agent

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// PortableAPIVersion is the YAML apiVersion for the exported Agent document.
const PortableAPIVersion = "agenthub.io/v1"

// PortableKind is the YAML kind for the exported Agent document.
const PortableKind = "Agent"

// PortableAgent is the canonical YAML/JSON envelope for an exported agent.
// Designed for versioning via git and sharing across tenants.
type PortableAgent struct {
	APIVersion string           `yaml:"apiVersion" json:"apiVersion"`
	Kind       string           `yaml:"kind" json:"kind"`
	Metadata   PortableMetadata `yaml:"metadata" json:"metadata"`
	Spec       PortableSpec     `yaml:"spec" json:"spec"`
}

// PortableMetadata holds identity fields preserved across export/import.
type PortableMetadata struct {
	Name        string `yaml:"name" json:"name"`
	Slug        string `yaml:"slug" json:"slug"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// PortableSpec is the agent configuration payload. Credentials inside ModelConfig
// are sanitized on export; tenants importing must re-inject their own credentials.
type PortableSpec struct {
	Instructions     string          `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	ModelConfig      json.RawMessage `yaml:"modelConfig,omitempty" json:"modelConfig,omitempty"`
	PermissionRules  json.RawMessage `yaml:"permissionRules,omitempty" json:"permissionRules,omitempty"`
	Config           json.RawMessage `yaml:"config,omitempty" json:"config,omitempty"`
	EnableManagement bool            `yaml:"enableManagement,omitempty" json:"enableManagement,omitempty"`
	SkillIDs         []uuid.UUID     `yaml:"skillIds,omitempty" json:"skillIds,omitempty"`
	KnowledgeBaseIDs []uuid.UUID     `yaml:"knowledgeBaseIds,omitempty" json:"knowledgeBaseIds,omitempty"`
}

// ToPortable converts an Agent + its bindings into a sharable PortableAgent.
// ModelConfig credentials are stripped via SanitizeModelConfig.
func ToPortable(a Agent, skillIDs, kbIDs []uuid.UUID) PortableAgent {
	instructions := ""
	if a.SystemPrompt != nil {
		instructions = *a.SystemPrompt
	}
	return PortableAgent{
		APIVersion: PortableAPIVersion,
		Kind:       PortableKind,
		Metadata: PortableMetadata{
			Name:        a.Name,
			Slug:        a.Slug,
			Description: a.Description,
		},
		Spec: PortableSpec{
			Instructions:     instructions,
			ModelConfig:      SanitizeModelConfig(a.ModelConfig),
			PermissionRules:  a.PermissionRules,
			Config:           a.Config,
			EnableManagement: a.EnableManagement,
			SkillIDs:         skillIDs,
			KnowledgeBaseIDs: kbIDs,
		},
	}
}

// ToCreateRequest adapts a PortableAgent into a CreateAgentRequest for import.
// Callers must validate skill/KB IDs exist before calling Service.Create.
func (p PortableAgent) ToCreateRequest() CreateAgentRequest {
	var sp *string
	if p.Spec.Instructions != "" {
		v := p.Spec.Instructions
		sp = &v
	}
	return CreateAgentRequest{
		Name:             p.Metadata.Name,
		Slug:             p.Metadata.Slug,
		Description:      p.Metadata.Description,
		SystemPrompt:     sp,
		ModelConfig:      p.Spec.ModelConfig,
		PermissionRules:  p.Spec.PermissionRules,
		Config:           p.Spec.Config,
		EnableManagement: p.Spec.EnableManagement,
		SkillIDs:         p.Spec.SkillIDs,
		KnowledgeBaseIDs: p.Spec.KnowledgeBaseIDs,
	}
}

// MarshalPortableYAML returns the YAML serialization of a PortableAgent.
func MarshalPortableYAML(p PortableAgent) ([]byte, error) {
	return yaml.Marshal(p)
}

// UnmarshalPortableYAML parses YAML bytes into a PortableAgent and validates the envelope.
func UnmarshalPortableYAML(data []byte) (PortableAgent, error) {
	var p PortableAgent
	if err := yaml.Unmarshal(data, &p); err != nil {
		return PortableAgent{}, fmt.Errorf("invalid yaml: %w", err)
	}
	if p.APIVersion != PortableAPIVersion {
		return PortableAgent{}, fmt.Errorf("unsupported apiVersion %q (expected %q)", p.APIVersion, PortableAPIVersion)
	}
	if p.Kind != PortableKind {
		return PortableAgent{}, fmt.Errorf("unsupported kind %q (expected %q)", p.Kind, PortableKind)
	}
	if p.Metadata.Name == "" || p.Metadata.Slug == "" {
		return PortableAgent{}, fmt.Errorf("metadata.name and metadata.slug are required")
	}
	return p, nil
}
