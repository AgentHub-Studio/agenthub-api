-- Seed capability-specific knowledge base templates in ah_core.knowledge_base_template
-- (migration 000096). These 3 templates are designed for the three capability agents
-- introduced in migration 000091 (core-researcher, core-analyst, core-planner).
-- They complement the 7 platform templates seeded in migration 000017.
--
-- Each template provides a pre-configured KB blueprint (chunk strategy + embedding
-- provider + doc types) so tenants can create a useful KB for a capability agent
-- in one click:
--
--   research-collection-template — curates web research findings (core-researcher)
--   analysis-workspace-template  — accepts docs for analysis (core-analyst)
--   project-notes-template       — stores project documentation (core-planner)
--
-- All 3:
--   - are is_recommended = TRUE (surfaces in the UI "Suggested" section)
--   - use local/e5-large embedding (matches platform default)
--   - use semantic chunk strategy (preserves narrative context)
--   - have sort_order ≥ 100 (above the 7 platform templates whose range is 10-70)
--
-- The ah_core.knowledge_base_template table is created by migration 000017.
-- New template_kind values (research_collection, analysis_workspace, project_notes)
-- are distinct from the 7 platform kinds (faq, documentation, internal_wiki,
-- chat_history, api_reference, regulatory, customer_support).

INSERT INTO ah_core.knowledge_base_template
    (slug, display_name, description, template_kind,
     default_embedding_provider_slug,
     chunk_strategy, chunk_size_tokens, chunk_overlap_tokens,
     recommended_top_k, supported_doc_types,
     requires_admin_review, is_recommended, sort_order)
VALUES

-- 1. Research Collection — for core-researcher agent.
('research-collection-template',
 'Research Collection',
 'A knowledge base for collecting and organizing web research findings, articles, and reference materials.',
 'research_collection',
 'local/e5-large',
 'semantic', 768, 100,
 5, 'md,html,txt,pdf',
 FALSE, TRUE, 100),

-- 2. Analysis Workspace — for core-analyst agent.
('analysis-workspace-template',
 'Analysis Workspace',
 'A knowledge base for uploading documents to be analyzed, compared, and synthesized by the Document Analyst.',
 'analysis_workspace',
 'local/e5-large',
 'semantic', 768, 100,
 5, 'pdf,docx,txt,md,html',
 FALSE, TRUE, 101),

-- 3. Project Notes — for core-planner agent.
('project-notes-template',
 'Project Notes',
 'A knowledge base for storing project documentation, requirements, meeting notes, and decision records for the Task Planner to reference.',
 'project_notes',
 'local/e5-large',
 'semantic', 768, 100,
 5, 'md,txt,docx,html,pdf',
 FALSE, TRUE, 102)

ON CONFLICT (slug) DO NOTHING;
