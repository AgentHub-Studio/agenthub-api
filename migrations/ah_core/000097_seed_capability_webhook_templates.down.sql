-- Rollback migration 000097: remove the 3 capability webhook notification templates.
-- The webhook_notification_template table itself is also dropped since it was created
-- by this migration (no prior migration seeded it).

DELETE FROM ah_core.webhook_notification_template
 WHERE slug IN (
     'capability-research-complete',
     'capability-analysis-done',
     'capability-tasks-updated'
 );

DROP TABLE IF EXISTS ah_core.webhook_notification_template;
