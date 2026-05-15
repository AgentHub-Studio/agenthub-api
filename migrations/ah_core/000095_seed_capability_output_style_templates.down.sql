-- Rollback migration 000095: remove the 3 capability output style templates.
-- The 8 platform styles seeded in migration 000011 are NOT touched.

DELETE FROM ah_core.output_style
 WHERE slug IN (
     'research-report',
     'analysis-brief',
     'task-checklist'
 );
