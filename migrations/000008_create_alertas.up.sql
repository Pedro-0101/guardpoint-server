CREATE TABLE IF NOT EXISTS alertas (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    turno_id        UUID        NOT NULL REFERENCES turnos(id),
    tipo            VARCHAR(30) NOT NULL,
    nivel           INTEGER     NOT NULL,
    status          VARCHAR(20) DEFAULT 'aberto',
    mensagem        TEXT,
    resolvido_em    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alertas_empresa ON alertas(empresa_id, status);
CREATE INDEX IF NOT EXISTS idx_alertas_turno ON alertas(turno_id);
