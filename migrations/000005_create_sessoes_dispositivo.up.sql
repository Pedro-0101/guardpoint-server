CREATE TABLE IF NOT EXISTS sessoes_dispositivo (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id  UUID        NOT NULL REFERENCES usuarios(id),
    empresa_id  UUID        NOT NULL REFERENCES empresas(id),
    device_id   VARCHAR(255) NOT NULL,
    criado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sessoes_dispositivo_usuario ON sessoes_dispositivo(usuario_id);
CREATE INDEX IF NOT EXISTS idx_sessoes_dispositivo_device ON sessoes_dispositivo(device_id);
