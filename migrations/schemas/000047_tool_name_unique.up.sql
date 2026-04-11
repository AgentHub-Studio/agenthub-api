-- Add unique constraint on tool.name within a schema/tenant.
-- P-C338-1 (ACT-F3-15): prevents duplicate tool names that confuse skill binding.
ALTER TABLE tool ADD CONSTRAINT uq_tool_name UNIQUE (name);
