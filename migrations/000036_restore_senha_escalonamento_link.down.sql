-- Recria as tabelas config_alerta_emergencia* e remove o vinculo
-- nivel_escalonamento_id de senhas_vigia.

CREATE TABLE config_alerta_emergencia (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL REFERENCES empresas(id) ON DELETE CASCADE,
    tipo       VARCHAR(20) NOT NULL CHECK (tipo IN ('sabotagem', 'no_show')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (empresa_id, tipo)
);

CREATE TABLE config_alerta_emergencia_destinatarios (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_alerta_emergencia_id   UUID NOT NULL REFERENCES config_alerta_emergencia(id) ON DELETE CASCADE,
    usuario_id                    UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (config_alerta_emergencia_id, usuario_id)
);

CREATE INDEX idx_config_alerta_emergencia_dest ON config_alerta_emergencia_destinatarios (config_alerta_emergencia_id);

DROP INDEX IF EXISTS uq_senhas_vigia_usuario_nivel;
ALTER TABLE senhas_vigia DROP CONSTRAINT IF EXISTS ck_senhas_vigia_nivel_obrigatorio;
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS nivel_escalonamento_id;
