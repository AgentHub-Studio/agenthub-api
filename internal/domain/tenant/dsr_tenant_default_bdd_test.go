package tenant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSR03DefaultRetrievalConfigMatchesADR014(t *testing.T) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(defaultTenantChatRetrievalConfig), &cfg); err != nil {
		t.Fatalf("defaultTenantChatRetrievalConfig is invalid JSON: %v", err)
	}

	if cfg["topK"] != float64(8) {
		t.Fatalf("topK = %v, want 8", cfg["topK"])
	}
	if cfg["driftThreshold"] != 0.55 {
		t.Fatalf("driftThreshold = %v, want 0.55", cfg["driftThreshold"])
	}
	if cfg["maxStickySize"] != float64(15) {
		t.Fatalf("maxStickySize = %v, want 15", cfg["maxStickySize"])
	}
	if cfg["allowDrift"] != true {
		t.Fatalf("allowDrift = %v, want true", cfg["allowDrift"])
	}
	if cfg["refreshPolicy"] != "drift_or_invalidation" {
		t.Fatalf("refreshPolicy = %v, want drift_or_invalidation", cfg["refreshPolicy"])
	}
	if cfg["minScore"] != 0.30 {
		t.Fatalf("minScore = %v, want 0.30", cfg["minScore"])
	}
}

func TestDSR03CreateTenantSeedsTenantChatDefaultIdempotently(t *testing.T) {
	for _, want := range []string{
		"INSERT INTO public.tenant_chat_default",
		"tenant_id",
		"name",
		"system_prompt",
		"model_config",
		"retrieval_config",
		"enable_management",
		"ON CONFLICT (tenant_id) DO NOTHING",
	} {
		if !strings.Contains(createTenantChatDefaultSQL, want) {
			t.Fatalf("createTenantChatDefaultSQL missing %q", want)
		}
	}
}

func TestDSR03MigrationBackfillsTenantChatDefaultPersonaFields(t *testing.T) {
	up := readPublicMigration(t, "000018_tenant_chat_default_persona.up.sql")
	down := readPublicMigration(t, "000018_tenant_chat_default_persona.down.sql")

	for _, want := range []string{
		"ADD COLUMN IF NOT EXISTS id UUID",
		"ADD COLUMN IF NOT EXISTS name VARCHAR(200) NOT NULL DEFAULT 'Assistente'",
		"ADD COLUMN IF NOT EXISTS system_prompt TEXT NOT NULL DEFAULT",
		"ADD COLUMN IF NOT EXISTS model_config JSONB NOT NULL DEFAULT",
		"ADD COLUMN IF NOT EXISTS retrieval_config JSONB NOT NULL DEFAULT",
		`"topK":8`,
		`"driftThreshold":0.55`,
		"UPDATE public.tenant_chat_default",
		"WHERE retrieval_config IS NULL",
		"OR retrieval_config = '{}'::jsonb",
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_chat_default_id",
	} {
		if !strings.Contains(up, want) {
			t.Fatalf("up migration missing %q", want)
		}
	}

	for _, want := range []string{
		"DROP INDEX IF EXISTS public.uq_tenant_chat_default_id",
		"DROP COLUMN IF EXISTS retrieval_config",
		"DROP COLUMN IF EXISTS system_prompt",
		"DROP COLUMN IF EXISTS id",
	} {
		if !strings.Contains(down, want) {
			t.Fatalf("down migration missing %q", want)
		}
	}
}

func readPublicMigration(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "migrations", "public", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(content)
}
