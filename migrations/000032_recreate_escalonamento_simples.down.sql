-- Rollback.

-- 1. Drop config_escalonamento table
ALTER TABLE config_escalonamento DROP CONSTRAINT IF EXISTS uq_config_escalonamento_empresa;
DROP TABLE IF EXISTS config_escalonamento_destinatarios;
DROP TABLE IF EXISTS config_escalonamento;

-- 2. Recreate columns in senhas_vigia
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS atraso_minutos INTEGER NOT NULL DEFAULT 0;
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS destinatarios UUID[] NOT NULL DEFAULT '{}';
