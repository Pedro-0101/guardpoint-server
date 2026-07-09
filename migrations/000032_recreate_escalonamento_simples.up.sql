-- Recreate config_escalonamento (sem nivel, sem sistema) e remove
-- atraso_minutos/destinatarios de senhas_vigia.

-- 1. Recreate config_escalonamento
CREATE TABLE IF NOT EXISTS config_escalonamento (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID NOT NULL REFERENCES empresas(id),
    atraso_minutos  INTEGER NOT NULL DEFAULT 15,
    descricao       VARCHAR(255) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 2. Recreate config_escalonamento_destinatarios
CREATE TABLE IF NOT EXISTS config_escalonamento_destinatarios (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_escalonamento_id  UUID NOT NULL REFERENCES config_escalonamento(id) ON DELETE CASCADE,
    usuario_id               UUID NOT NULL REFERENCES usuarios(id),
    UNIQUE(config_escalonamento_id, usuario_id)
);

-- 3. Remove old columns from senhas_vigia (added by migration 000031 which was reverted)
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS atraso_minutos;
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS destinatarios;

-- 4. Ensure config_escalonamento has UNIQUE(empresa_id)
ALTER TABLE config_escalonamento ADD CONSTRAINT uq_config_escalonamento_empresa UNIQUE (empresa_id);
