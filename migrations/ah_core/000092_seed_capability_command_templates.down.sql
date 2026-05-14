-- Remove capability slash commands seeded in migration 000092.
DELETE FROM ah_core.command
 WHERE slug IN ('research', 'analyze', 'plan', 'summarize-doc', 'tasks');
