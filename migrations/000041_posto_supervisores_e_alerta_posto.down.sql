DROP INDEX IF EXISTS idx_alertas_posto;
ALTER TABLE alertas DROP COLUMN IF EXISTS posto_id;
DROP TABLE IF EXISTS posto_supervisores;
