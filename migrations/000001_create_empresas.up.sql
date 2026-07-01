CREATE TABLE IF NOT EXISTS empresas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome        VARCHAR(255) NOT NULL,
    cnpj        VARCHAR(14)  NOT NULL UNIQUE,
    ativa       BOOLEAN      DEFAULT true,
    created_at  TIMESTAMPTZ  DEFAULT now()
);
