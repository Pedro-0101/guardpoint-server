CREATE TABLE IF NOT EXISTS checkins (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    turno_id            UUID        NOT NULL REFERENCES turnos(id),
    empresa_id          UUID        NOT NULL REFERENCES empresas(id),
    latitude            DOUBLE PRECISION NOT NULL,
    longitude           DOUBLE PRECISION NOT NULL,
    timestamp_criacao   TIMESTAMPTZ NOT NULL,
    timestamp_recebimento TIMESTAMPTZ DEFAULT now(),
    tipo_senha          VARCHAR(20) NOT NULL,
    flag_geofence       VARCHAR(20),
    origem_rede         VARCHAR(20) DEFAULT 'online',
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_checkins_turno ON checkins(turno_id, timestamp_criacao);
CREATE INDEX IF NOT EXISTS idx_checkins_empresa ON checkins(empresa_id);
