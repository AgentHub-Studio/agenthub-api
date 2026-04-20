-- Backfill current_version on ah_core specialist agents.
-- Migration 000006 inserts 12 specialists without current_version, leaving it
-- NULL. The agent repository scans current_version into *int (not nullable),
-- so listing agents in the core tenant fails with:
--   "can't scan into dest[5] (col: current_version): cannot scan NULL into *int"

UPDATE ah_core.agent SET current_version = 1 WHERE current_version IS NULL;
