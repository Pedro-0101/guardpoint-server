-- Reverte: remove coluna sistema e restaura UNIQUE(empresa_id)

ALTER TABLE config_escalonamento DROP COLUMN IF EXISTS sistema;
-- Pode falhar se houver múltiplos registros por empresa; é esperado.
ALTER TABLE config_escalonamento ADD CONSTRAINT uq_config_escalonamento_empresa UNIQUE (empresa_id);
