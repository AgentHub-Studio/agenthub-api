-- Seed capability-specific output styles in ah_core.output_style (migration 000095).
-- These 3 styles are response-formatting templates designed for the three capability
-- agents introduced in migration 000091 (core-researcher, core-analyst, core-planner).
-- They complement the 8 platform styles seeded in migration 000011 (conversational,
-- concise, structured, technical, verbose, tutorial, executive, json_only).
--
-- Each style encodes the canonical output structure for its target agent:
--   research-report  — structured web research output (core-researcher)
--   analysis-brief   — document analysis with evidence base (core-analyst)
--   task-checklist   — numbered task plan with checkboxes (core-planner)
--
-- All 3 use output_format='markdown', audience='general', sort_order ≥ 100
-- (above the platform styles whose sort_order range is 10–80).
--
-- Inspired by Claude Code's output formatting approach (PDF arXiv:2604.14228v1).
-- The ah_core.output_style table is created by migration 000011.

INSERT INTO ah_core.output_style
    (slug, name, description, prompt_template, output_format, max_words, audience,
     is_default, is_active, sort_order)
VALUES

-- 1. Research Report — for core-researcher agent.
('research-report',
 'Research Report',
 'Structured web research output with findings, sources, and recommendations.',
 'Format your response as a Research Report with these sections:' || E'\n' ||
 '## Key Findings' || E'\n' ||
 '(3-5 bullet points with the most important discoveries)' || E'\n\n' ||
 '## Sources' || E'\n' ||
 '(List all URLs with titles and access dates)' || E'\n\n' ||
 '## Analysis' || E'\n' ||
 '(Synthesis of findings with confidence levels)' || E'\n\n' ||
 '## Recommended Next Steps' || E'\n' ||
 '(2-3 actionable follow-up items)' || E'\n\n' ||
 'Always cite sources inline using [Source N] notation.',
 'markdown', 800, 'general', FALSE, TRUE, 100),

-- 2. Analysis Brief — for core-analyst agent.
('analysis-brief',
 'Analysis Brief',
 'Structured document analysis with executive summary, evidence, and insights.',
 'Format your response as an Analysis Brief with these sections:' || E'\n' ||
 '## Executive Summary' || E'\n' ||
 '(2-3 sentences capturing the essential answer)' || E'\n\n' ||
 '## Key Themes' || E'\n' ||
 '(Bulleted list with supporting evidence and document citations)' || E'\n\n' ||
 '## Evidence Base' || E'\n' ||
 '(List documents referenced with their relevance)' || E'\n\n' ||
 '## Gaps & Caveats' || E'\n' ||
 '(What the documents don''t cover or contradict)' || E'\n\n' ||
 '## Insights' || E'\n' ||
 '(Actionable conclusions drawn from the analysis)' || E'\n\n' ||
 'Cite documents as [Doc: title] inline.',
 'markdown', 1000, 'general', FALSE, TRUE, 101),

-- 3. Task Checklist — for core-planner agent.
('task-checklist',
 'Task Checklist',
 'Structured task plan with numbered steps, effort estimates, and dependencies.',
 'Format your response as a Task Plan:' || E'\n' ||
 '## Goal' || E'\n' ||
 '(One-sentence restatement of the objective)' || E'\n\n' ||
 '## Tasks' || E'\n' ||
 '(Numbered list: each task has a clear action verb, estimated effort in hours/days, and any dependencies)' || E'\n\n' ||
 '## Timeline' || E'\n' ||
 '(High-level sequence showing task order and critical path)' || E'\n\n' ||
 '## Success Criteria' || E'\n' ||
 '(How to know when each major milestone is complete)' || E'\n\n' ||
 'Use [ ] checkboxes for each task so progress can be tracked.',
 'markdown', 600, 'general', FALSE, TRUE, 102)

ON CONFLICT (slug) DO NOTHING;
