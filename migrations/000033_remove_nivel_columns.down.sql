ALTER TABLE config_escalonamento DROP CONSTRAINT IF EXISTS uq_config_escalonamento_empresa;
ALTER TABLE config_escalonamento ADD COLUMN IF NOT EXISTS nivel INTEGER NOT NULL DEFAULT 1;
ALTER TABLE config_escalonamento ADD COLUMN IF NOT EXISTS sistema BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE config_escalonamento ADD UNIQUE (empresa_id, nivel);

ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS nivel_escalonamento_id UUID;
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS atraso_minutos INTEGER NOT NULL DEFAULT 0;
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS destinatarios UUID[] NOT NULL DEFAULT '{}';
