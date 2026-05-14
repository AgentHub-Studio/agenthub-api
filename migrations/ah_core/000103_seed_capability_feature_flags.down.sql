-- Down migration for 000103: remove capability feature flag rows
-- and drop the capability_feature_flag table.
--
-- Removes the 6 rows seeded by 000103.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_feature_flag
WHERE slug IN (
    'capability-citations',
    'capability-task-tracking',
    'capability-kb-indexing',
    'capability-subagent-delegation',
    'capability-doc-citations',
    'capability-progressive-summarization'
);

DROP TABLE IF EXISTS ah_core.capability_feature_flag;
