SET search_path TO ah_core, public;

-- Rollback for migration 000125 (capability suggested follow-ups).
-- Removes the 9 seeded follow-up prompts for the three capability agents and
-- drops the capability_suggested_followup table.

DELETE FROM capability_suggested_followup
 WHERE (agent_slug, followup_slug) IN (
     ('core-researcher', 'research-current-sources'),
     ('core-researcher', 'compare-official-and-recent'),
     ('core-researcher', 'fact-check-claim'),
     ('core-analyst',    'extract-key-patterns'),
     ('core-analyst',    'compare-options'),
     ('core-analyst',    'turn-findings-into-actions'),
     ('core-planner',    'break-into-milestones'),
     ('core-planner',    'identify-risks-and-owners'),
     ('core-planner',    'weekly-checklist')
 );

DROP TABLE IF EXISTS capability_suggested_followup;
