-- Remove colunas nivel de config_escalonamento e nivel_escalonamento_id de senhas_vigia.

ALTER TABLE config_escalonamento DROP COLUMN IF EXISTS nivel;
ALTER TABLE config_escalonamento DROP COLUMN IF EXISTS sistema;
ALTER TABLE config_escalonamento DROP CONSTRAINT IF EXISTS uq_config_escalonamento_empresa;
ALTER TABLE config_escalonamento ADD CONSTRAINT uq_config_escalonamento_empresa UNIQUE (empresa_id);

ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS nivel_escalonamento_id;
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS atraso_minutos;
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS destinatarios;
