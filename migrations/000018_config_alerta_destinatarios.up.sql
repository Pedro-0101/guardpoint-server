-- migrations/000018_config_alerta_destinatarios.up.sql
ALTER TABLE config_escalonamento DROP COLUMN whatsapp_para;
ALTER TABLE config_escalonamento DROP COLUMN cargo_alvo;

CREATE TABLE config_escalonamento_destinatarios (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_escalonamento_id UUID NOT NULL REFERENCES config_escalonamento(id) ON DELETE CASCADE,
    usuario_id              UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at              TIMESTAMPTZ DEFAULT now(),
    UNIQUE(config_escalonamento_id, usuario_id)
);

CREATE INDEX IF NOT EXISTS idx_config_escalonamento_destinatarios_config
    ON config_escalonamento_destinatarios(config_escalonamento_id);

CREATE TABLE config_alerta_emergencia (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id  UUID NOT NULL REFERENCES empresas(id),
    tipo        VARCHAR(20) NOT NULL CHECK (tipo IN ('coacao', 'sabotagem', 'no_show')),
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(empresa_id, tipo)
);

CREATE TABLE config_alerta_emergencia_destinatarios (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_alerta_emergencia_id UUID NOT NULL REFERENCES config_alerta_emergencia(id) ON DELETE CASCADE,
    usuario_id                  UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at                  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(config_alerta_emergencia_id, usuario_id)
);

CREATE INDEX IF NOT EXISTS idx_config_alerta_emergencia_destinatarios_config
    ON config_alerta_emergencia_destinatarios(config_alerta_emergencia_id);
