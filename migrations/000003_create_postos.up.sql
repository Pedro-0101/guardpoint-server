CREATE TABLE IF NOT EXISTS postos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id  UUID         NOT NULL REFERENCES empresas(id),
    nome        VARCHAR(255) NOT NULL,
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    raio_m      INTEGER      DEFAULT 100,
    ativo       BOOLEAN      DEFAULT true,
    created_at  TIMESTAMPTZ  DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_postos_empresa ON postos(empresa_id);
