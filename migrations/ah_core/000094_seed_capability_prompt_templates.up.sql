-- Seed capability-specific prompt templates in ah_core.prompt_template (migration 000094).
-- These 5 templates are user-facing starters that help users compose requests to
-- the capability agents seeded in migration 000091 (core-researcher, core-analyst,
-- core-planner). They complement the 8 generic platform templates in migration 000019.
--
-- Template kind = 'capability' distinguishes this set from the generic platform
-- templates (assistant, coder, analyst, etc.) and enables the capability layer to
-- be queried independently. All 5 are marked is_recommended = TRUE.
--
-- The prompt_template table is created by migration 000019_seed_prompt_templates.up.sql.

INSERT INTO ah_core.prompt_template (slug, display_name, description, template_kind,
    system_prompt, recommended_temperature, recommended_max_tokens,
    requires_tools, placeholders, is_recommended)
VALUES

-- 1. Web Research Brief — compose a structured research brief via web search.
('capability-web-research-brief',
 'Web Research Brief',
 'Compose a structured research brief on any topic using live web search. Returns key findings, credible sources with URLs, publication dates, and recommended next steps. Ideal starting point for the core-researcher capability agent.',
 'capability',
 'Research the following topic using web search and return a structured brief with: key findings, credible sources (with URLs), publication dates, and recommended next steps.' || E'\n\n' ||
 'Topic: {{topic}}' || E'\n\n' ||
 'Focus areas: {{focus_areas}}' || E'\n' ||
 'Desired depth: {{depth}}',
 0.30, 4096,
 'core-web-research',
 'topic,focus_areas,depth',
 TRUE),

-- 2. Document Analysis Summary — search knowledge base and synthesise findings.
('capability-doc-analysis-summary',
 'Document Analysis Summary',
 'Search the knowledge base for documents on a topic and produce a comprehensive analysis: executive summary, key themes, supporting evidence with document citations, contradictions or gaps, and actionable insights. Ideal for the core-analyst capability agent.',
 'capability',
 'Search the knowledge base for documents related to the following topic and provide a comprehensive analysis including: executive summary, key themes, supporting evidence (with document citations), contradictions or gaps, and actionable insights.' || E'\n\n' ||
 'Topic: {{topic}}' || E'\n\n' ||
 'Document scope: {{scope}}',
 0.40, 6000,
 'core-doc-analysis',
 'topic,scope',
 TRUE),

-- 3. Task Breakdown Plan — decompose a goal into a sequenced, actionable task list.
('capability-task-breakdown',
 'Task Breakdown Plan',
 'Break down any goal into a detailed, sequenced task list with clear descriptions, estimated effort, dependencies, and success criteria organised by priority. Ideal for the core-planner capability agent.',
 'capability',
 'Break down the following goal into a detailed, actionable task list. Create tasks with clear descriptions, estimated effort, dependencies, and success criteria. Organize by priority and sequence.' || E'\n\n' ||
 'Goal: {{goal}}' || E'\n\n' ||
 'Constraints: {{constraints}}' || E'\n' ||
 'Timeline: {{timeline}}',
 0.50, 4096,
 'core-task-workflow',
 'goal,constraints,timeline',
 TRUE),

-- 4. Competitive Research — web-based competitive landscape report.
('capability-competitive-research',
 'Competitive Research',
 'Conduct competitive research on any subject using live web search. Compares key players, identifies differentiators, and delivers a structured competitive landscape report. Uses the core-web-research skill.',
 'capability',
 'Conduct competitive research on the following subject. Search the web for current market information, compare key players, identify differentiators, and summarize findings in a structured report.' || E'\n\n' ||
 'Subject: {{subject}}' || E'\n\n' ||
 'Competitors to analyse: {{competitors}}' || E'\n' ||
 'Key criteria: {{criteria}}',
 0.30, 5000,
 'core-web-research',
 'subject,competitors,criteria',
 TRUE),

-- 5. Knowledge Base Synthesis — synthesise an answer from knowledge base documents.
('capability-knowledge-synthesis',
 'Knowledge Base Synthesis',
 'Search the knowledge base for all documents relevant to a question, synthesise the information into a coherent answer, and highlight any gaps or inconsistencies found. Uses the core-doc-analysis skill.',
 'capability',
 'Search the knowledge base for all documents relevant to the following question, synthesise the information into a coherent answer, and highlight any gaps or inconsistencies found.' || E'\n\n' ||
 'Question: {{question}}' || E'\n\n' ||
 'Context: {{context}}',
 0.40, 5000,
 'core-doc-analysis',
 'question,context',
 TRUE)

ON CONFLICT (slug) DO NOTHING;
