CREATE TABLE IF NOT EXISTS turnos (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    usuario_id      UUID        NOT NULL REFERENCES usuarios(id),
    posto_id        UUID        NOT NULL REFERENCES postos(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'agendado',
    inicio_previsto TIMESTAMPTZ NOT NULL,
    fim_previsto    TIMESTAMPTZ NOT NULL,
    inicio_real     TIMESTAMPTZ,
    fim_real        TIMESTAMPTZ,
    token_sessao    VARCHAR(255),
    intervalo_min   INTEGER     DEFAULT 30,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_turnos_empresa ON turnos(empresa_id);
CREATE INDEX IF NOT EXISTS idx_turnos_usuario ON turnos(usuario_id);
CREATE INDEX IF NOT EXISTS idx_turnos_status ON turnos(empresa_id, status);
