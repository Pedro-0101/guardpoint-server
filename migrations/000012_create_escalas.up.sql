CREATE TABLE escalas (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID NOT NULL REFERENCES empresas(id),
    usuario_id      UUID NOT NULL REFERENCES usuarios(id),
    posto_id        UUID NOT NULL REFERENCES postos(id),
    data_inicio     DATE NOT NULL,
    data_fim        DATE NOT NULL,
    hora_inicio     TIME NOT NULL,
    hora_fim        TIME NOT NULL,
    dias_semana     SMALLINT[] NOT NULL DEFAULT '{0,1,2,3,4,5,6}',
    tolerancia_min  INTEGER NOT NULL DEFAULT 15,
    ativo           BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_escalas_datas CHECK (data_fim >= data_inicio),
    CONSTRAINT ck_escalas_horas CHECK (hora_fim > hora_inicio)
);

CREATE INDEX idx_escalas_empresa ON escalas(empresa_id);
CREATE INDEX idx_escalas_usuario ON escalas(usuario_id);
CREATE INDEX idx_escalas_posto ON escalas(posto_id);
CREATE INDEX idx_escalas_datas ON escalas(data_inicio, data_fim);
