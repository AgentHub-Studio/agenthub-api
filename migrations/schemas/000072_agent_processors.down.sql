ALTER TABLE agent
  DROP COLUMN IF EXISTS output_processors,
  DROP COLUMN IF EXISTS input_processors;
