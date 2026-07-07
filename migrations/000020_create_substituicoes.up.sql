CREATE TABLE substituicoes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID NOT NULL REFERENCES empresas(id),
    usuario_id      UUID NOT NULL REFERENCES usuarios(id),
    posto_id        UUID NOT NULL REFERENCES postos(id),
    data_inicio     DATE NOT NULL,
    data_fim        DATE NOT NULL,
    hora_inicio     TIME NOT NULL,
    hora_fim        TIME NOT NULL,
    tolerancia_min  INTEGER NOT NULL DEFAULT 15,
    motivo          VARCHAR(255),
    ativo           BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_substituicoes_datas CHECK (data_fim >= data_inicio),
    CONSTRAINT ck_substituicoes_horas CHECK (hora_fim <> hora_inicio)
);

CREATE INDEX idx_substituicoes_empresa ON substituicoes(empresa_id);
CREATE INDEX idx_substituicoes_usuario ON substituicoes(usuario_id);
CREATE INDEX idx_substituicoes_posto ON substituicoes(posto_id);
CREATE INDEX idx_substituicoes_datas ON substituicoes(data_inicio, data_fim);
