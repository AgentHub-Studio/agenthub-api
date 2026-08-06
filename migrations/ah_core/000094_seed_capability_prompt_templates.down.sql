-- Rollback migration 000094: remove the 5 capability prompt templates.
-- The 8 platform templates seeded in migration 000019 are NOT touched.

DELETE FROM ah_core.core_prompt_template
 WHERE slug IN (
     'capability-web-research-brief',
     'capability-doc-analysis-summary',
     'capability-task-breakdown',
     'capability-competitive-research',
     'capability-knowledge-synthesis'
 );
