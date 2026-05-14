-- Down migration for 000106: remove capability onboarding checklist rows
-- and drop the capability_onboarding_checklist table.
--
-- Removes the 5 rows seeded by 000106.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_onboarding_checklist
WHERE slug IN (
    'onboarding-connect-llm',
    'onboarding-create-agent',
    'onboarding-assign-skill',
    'onboarding-configure-kb',
    'onboarding-test-run'
);

DROP TABLE IF EXISTS ah_core.capability_onboarding_checklist;
