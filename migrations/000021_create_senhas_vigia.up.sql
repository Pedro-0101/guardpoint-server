CREATE TABLE senhas_vigia (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              UUID NOT NULL REFERENCES empresas(id),
    usuario_id              UUID NOT NULL REFERENCES usuarios(id),
    tipo                    VARCHAR(20) NOT NULL CHECK (tipo IN ('ok', 'emergencia', 'customizada')),
    codigo                  VARCHAR(6)  NOT NULL CHECK (codigo ~ '^[0-9]{4,6}$'),
    descricao               VARCHAR(255),
    nivel_escalonamento_id  UUID REFERENCES config_escalonamento(id) ON DELETE RESTRICT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_senhas_vigia_descricao_customizada
        CHECK (tipo <> 'customizada' OR descricao IS NOT NULL),
    CONSTRAINT ck_senhas_vigia_nivel_fixo_so_customizada
        CHECK (tipo = 'customizada' OR nivel_escalonamento_id IS NULL),
    UNIQUE (usuario_id, codigo)
);

-- Garante exatamente 1 PIN 'ok' e 1 'emergencia' por vigia; 'customizada' livre (0..N).
CREATE UNIQUE INDEX uq_senhas_vigia_ok         ON senhas_vigia(usuario_id) WHERE tipo = 'ok';
CREATE UNIQUE INDEX uq_senhas_vigia_emergencia ON senhas_vigia(usuario_id) WHERE tipo = 'emergencia';

CREATE INDEX idx_senhas_vigia_empresa ON senhas_vigia(empresa_id);
CREATE INDEX idx_senhas_vigia_nivel   ON senhas_vigia(nivel_escalonamento_id) WHERE nivel_escalonamento_id IS NOT NULL;
