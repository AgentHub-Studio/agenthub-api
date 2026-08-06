-- Enforce the A/B lifecycle defined by docs/SPEC.md. Older installations
-- could persist arbitrary status values because 000056 declared no CHECK.
-- Map those inactive, undefined states to PAUSED before enforcing the enum so
-- the migration preserves their no-routing behavior and lets an administrator
-- choose the proper lifecycle state explicitly.
UPDATE agent_ab_test
SET status = 'PAUSED'
WHERE status NOT IN ('ACTIVE', 'PAUSED', 'CONCLUDED');

ALTER TABLE agent_ab_test
    ADD CONSTRAINT agent_ab_test_status_check
    CHECK (status IN ('ACTIVE', 'PAUSED', 'CONCLUDED'));
