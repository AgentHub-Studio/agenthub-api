-- Down migration for 000107: remove capability UI hint rows
-- and drop the capability_ui_hint table.
--
-- Removes the 6 rows seeded by 000107.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_ui_hint
WHERE slug IN (
    'hint-researcher-start-tip',
    'hint-researcher-citation-tip',
    'hint-analyst-doc-upload-tip',
    'hint-analyst-confidence-tip',
    'hint-planner-task-tip',
    'hint-planner-breakdown-tip'
);

DROP TABLE IF EXISTS ah_core.capability_ui_hint;
