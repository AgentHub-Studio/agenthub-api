-- Seed platform-managed DEFAULT OUTPUT STYLES in ah_core.
-- Output styles are response-formatting templates the runner can switch
-- between. Tenants pick a default per agent; users override per session.
-- Inspired by Claude Code's output styles concept (PDF Section 6.1, 7).
--
-- An output style is a STYLE TEMPLATE that gets prepended to the system
-- prompt to shape the response: structure, length, register, formatting.
-- It is NOT the agent persona — multiple agents can share one style.
--
-- The platform seeds 8 styles covering the common web-product needs:
--   conversational  — default chat tone
--   concise         — under 150 words, no preamble
--   structured      — markdown headers + bullet lists
--   technical       — code-blocks first, prose minimal
--   verbose         — full reasoning + examples + caveats
--   tutorial        — step-by-step, numbered, beginner audience
--   executive       — TL;DR + 3 bullets, decision-oriented
--   json_only       — strict JSON envelope (for tool-pipeline use)

CREATE TABLE IF NOT EXISTS ah_core.output_style (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    -- slug is the stable identifier for this default style.
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    description     TEXT,
    -- prompt_template is the directive prepended to the system prompt.
    prompt_template TEXT         NOT NULL,
    -- output_format hints the renderer (markdown / plain / json).
    output_format   VARCHAR(32)  NOT NULL DEFAULT 'markdown',
    -- max_words is a soft hint to the LLM (0 = unbounded).
    max_words       INTEGER      NOT NULL DEFAULT 0,
    -- audience identifies the target reader (general / technical /
    -- executive / beginner). Surfaces in the UI picker.
    audience        VARCHAR(32)  NOT NULL DEFAULT 'general',
    -- is_default marks the style auto-selected for new agents/sessions.
    -- AT MOST ONE row may have is_default=TRUE (UNIQUE partial index).
    is_default      BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_output_style_slug      ON ah_core.output_style (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_output_style_is_active ON ah_core.output_style (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_output_style_format    ON ah_core.output_style (output_format);

-- AT MOST ONE default style — enforced by partial unique index.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_ah_core_output_style_default
    ON ah_core.output_style (is_default)
    WHERE is_default = TRUE;

-- Conversational — default
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('conversational',
 'Conversational',
 'Default chat tone — natural, friendly, suitable for most user interactions.',
 'Respond conversationally. Use a friendly, professional tone. Use markdown for emphasis when helpful, but do not over-format. Match the user register.',
 'markdown', 0, 'general', TRUE, 10);

-- Concise — under 150 words
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('concise',
 'Concise',
 'Under 150 words. No preamble. Direct answers only.',
 'Respond in under 150 words. Skip preamble (no "Sure!" or "Of course"). Skip restating the question. Get directly to the answer. If the answer is a list, use bullets without intro. If the answer is a single value, return just the value.',
 'markdown', 150, 'general', FALSE, 20);

-- Structured — markdown headers + bullets
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('structured',
 'Structured',
 'Markdown with clear headers, bullet lists, organized sections.',
 'Structure the response with markdown: ## headers for sections, - bullet lists for items, **bold** for key terms. Always have at least 2 sections. Prefer lists over prose for enumerable content.',
 'markdown', 0, 'general', FALSE, 30);

-- Technical — code-first
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('technical',
 'Technical',
 'Code blocks first, prose minimal. For developer/engineer audiences.',
 'Lead with the code or technical artifact. Wrap all code in fenced blocks with language tags. Keep prose to a minimum — explain only WHY, not WHAT (the code self-documents). Cite file paths as `path:line` when referencing existing code.',
 'markdown', 0, 'technical', FALSE, 40);

-- Verbose — full reasoning
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('verbose',
 'Verbose',
 'Full reasoning, examples, caveats. For learning or auditable contexts.',
 'Provide full reasoning: state assumptions, walk through the logic step by step, give 1-2 worked examples, list caveats and edge cases. Use markdown sections. This style is for learning/auditing — do NOT use for routine answers.',
 'markdown', 0, 'general', FALSE, 50);

-- Tutorial — step-by-step
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('tutorial',
 'Tutorial',
 'Numbered step-by-step, beginner audience, includes background context.',
 'Format as a numbered tutorial: 1, 2, 3. Begin with a 1-2 sentence "What you will build" intro. Each step must be self-contained — beginner can follow without prior context. Conclude with "Next steps" pointing to related skills.',
 'markdown', 0, 'beginner', FALSE, 60);

-- Executive — TL;DR + bullets
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('executive',
 'Executive',
 'TL;DR + 3 key bullets + recommended action. Decision-oriented.',
 'Format: (1) **TL;DR:** one sentence. (2) **Key points:** exactly 3 bullets. (3) **Recommended action:** one sentence the reader can act on now. No background, no caveats — assume executive context where time matters.',
 'markdown', 200, 'executive', FALSE, 70);

-- JSON only — strict envelope
INSERT INTO ah_core.output_style (slug, name, description, prompt_template, output_format, max_words, audience, is_default, sort_order) VALUES
('json_only',
 'JSON Only',
 'Strict JSON envelope. No prose, no markdown, no code fences. For tool-pipeline integration.',
 'Respond with VALID JSON only. No prose, no markdown, no code fences, no explanation. Top-level object with keys appropriate to the question. If the answer is a list, return {"items":[...]}. If you cannot answer, return {"error":"...", "reason":"..."}.',
 'json', 0, 'technical', FALSE, 80);
