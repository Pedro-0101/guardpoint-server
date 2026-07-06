ALTER TABLE empresas
  DROP COLUMN IF EXISTS alerta_sonoro,
  DROP COLUMN IF EXISTS updated_at;
