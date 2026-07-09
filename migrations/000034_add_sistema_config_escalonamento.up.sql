-- Adiciona coluna sistema à config_escalonamento e remove UNIQUE(empresa_id)
-- para permitir múltiplos escalonamentos customizados por empresa.

ALTER TABLE config_escalonamento ADD COLUMN IF NOT EXISTS sistema BOOLEAN NOT NULL DEFAULT false;

-- Marca o registro existente como sistema=true
UPDATE config_escalonamento SET sistema = true WHERE sistema = false;

-- Remove UNIQUE(empresa_id) para permitir múltiplos registros por empresa
ALTER TABLE config_escalonamento DROP CONSTRAINT IF EXISTS uq_config_escalonamento_empresa;
