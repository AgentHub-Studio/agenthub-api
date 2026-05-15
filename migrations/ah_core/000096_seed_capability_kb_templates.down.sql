-- Rollback migration 000096: remove the 3 capability KB templates.
-- The 7 platform templates (migration 000017) are NOT affected.

DELETE FROM ah_core.knowledge_base_template
 WHERE slug IN (
     'research-collection-template',
     'analysis-workspace-template',
     'project-notes-template'
 );
