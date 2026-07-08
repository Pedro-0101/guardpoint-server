-- Rollback: recria o sistema de niveis de escalonamento.
-- ATENCAO: a migracao reversa pode perder dados se destinatarios foram modificados
-- apos a migracao de ida (000031), pois nao e possivel reconstruir a FK corretamente.

-- 1. Recriar tabela config_escalonamento
CREATE TABLE IF NOT EXISTS config_escalonamento (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID NOT NULL REFERENCES empresas(id),
    nivel           INTEGER NOT NULL,
    atraso_minutos  INTEGER NOT NULL,
    sistema         BOOLEAN NOT NULL DEFAULT false,
    descricao       VARCHAR(255) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(empresa_id, nivel)
);

CREATE INDEX IF NOT EXISTS idx_config_escalonamento_empresa ON config_escalonamento(empresa_id, nivel);

-- 2. Recriar tabela config_escalonamento_destinatarios
CREATE TABLE IF NOT EXISTS config_escalonamento_destinatarios (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_escalonamento_id  UUID NOT NULL REFERENCES config_escalonamento(id) ON DELETE CASCADE,
    usuario_id               UUID NOT NULL REFERENCES usuarios(id),
    UNIQUE(config_escalonamento_id, usuario_id)
);

CREATE INDEX IF NOT EXISTS idx_config_escalonamento_destinatarios_config
    ON config_escalonamento_destinatarios(config_escalonamento_id);

-- 3. Recriar nivel padrao para cada empresa (sistema)
INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos, sistema, descricao)
SELECT DISTINCT e.id, 1, 0, true, 'Emergencia nao especificada'
FROM empresas e
WHERE NOT EXISTS (SELECT 1 FROM config_escalonamento c WHERE c.empresa_id = e.id);

-- Destinatarios dos niveis sistema = admins da empresa
INSERT INTO config_escalonamento_destinatarios (config_escalonamento_id, usuario_id)
SELECT c.id, u.id
FROM config_escalonamento c
JOIN empresas e ON e.id = c.empresa_id
JOIN usuarios u ON u.empresa_id = e.id AND u.role = 'admin'
WHERE c.sistema = true
  AND NOT EXISTS (
      SELECT 1 FROM config_escalonamento_destinatarios d
      WHERE d.config_escalonamento_id = c.id AND d.usuario_id = u.id
  );

-- 4. Recriar coluna nivel_escalonamento_id em senhas_vigia
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS nivel_escalonamento_id UUID;

-- Tentativa de relink: cria um nivel customizado por senha (nivel = row_number)
-- e vincula. Dados podem ser perdidos se nao houver config_escalonamento viavel.
UPDATE senhas_vigia sv
SET nivel_escalonamento_id = (
    SELECT c.id FROM config_escalonamento c
    WHERE c.empresa_id = sv.empresa_id AND c.sistema = true
    LIMIT 1
)
WHERE sv.tipo IN ('emergencia', 'customizada')
  AND sv.destinatarios IS NOT NULL AND array_length(sv.destinatarios, 1) > 0;

-- 5. Recriar constraints e indices
ALTER TABLE senhas_vigia ADD CONSTRAINT senhas_vigia_nivel_escalonamento_id_fkey
    FOREIGN KEY (nivel_escalonamento_id) REFERENCES config_escalonamento(id) ON DELETE RESTRICT;

ALTER TABLE senhas_vigia ADD CONSTRAINT ck_senhas_vigia_nivel_obrigatorio
    CHECK (tipo = 'ok' OR nivel_escalonamento_id IS NOT NULL);

CREATE INDEX idx_senhas_vigia_nivel
    ON senhas_vigia(nivel_escalonamento_id) WHERE nivel_escalonamento_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_senhas_vigia_usuario_nivel
    ON senhas_vigia(usuario_id, nivel_escalonamento_id) WHERE nivel_escalonamento_id IS NOT NULL;

-- 6. Remover colunas novas
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS destinatarios;
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS atraso_minutos;
