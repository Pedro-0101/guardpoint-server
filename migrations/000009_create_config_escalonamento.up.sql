CREATE TABLE IF NOT EXISTS config_escalonamento (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    nivel           INTEGER     NOT NULL,
    atraso_minutos  INTEGER     NOT NULL,
    whatsapp_para   VARCHAR(20) NOT NULL,
    cargo_alvo      VARCHAR(50),
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(empresa_id, nivel)
);

CREATE INDEX IF NOT EXISTS idx_config_escalonamento_empresa ON config_escalonamento(empresa_id, nivel);
