CREATE TABLE IF NOT EXISTS posto_supervisores (
    posto_id      UUID NOT NULL REFERENCES postos(id) ON DELETE CASCADE,
    supervisor_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (posto_id, supervisor_id)
);

ALTER TABLE alertas ADD COLUMN IF NOT EXISTS posto_id UUID REFERENCES postos(id);
CREATE INDEX IF NOT EXISTS idx_alertas_posto ON alertas(posto_id);
