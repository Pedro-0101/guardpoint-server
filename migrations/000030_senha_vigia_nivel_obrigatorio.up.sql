-- Remove a constraint antiga que so permitia nivel_escalonamento_id em senhas customizada.
ALTER TABLE senhas_vigia DROP CONSTRAINT IF EXISTS ck_senhas_vigia_nivel_fixo_so_customizada;

-- Nova constraint: apenas o tipo "ok" pode ter nivel_escalonamento_id NULL.
-- "emergencia" e "customizada" agora exigem nivel_escalonamento_id obrigatorio.
ALTER TABLE senhas_vigia ADD CONSTRAINT ck_senhas_vigia_nivel_obrigatorio
    CHECK (tipo = 'ok' OR nivel_escalonamento_id IS NOT NULL);

-- Um mesmo nivel de escalonamento nao pode ser usado por duas senhas do mesmo vigia.
CREATE UNIQUE INDEX IF NOT EXISTS uq_senhas_vigia_usuario_nivel
    ON senhas_vigia(usuario_id, nivel_escalonamento_id)
    WHERE nivel_escalonamento_id IS NOT NULL;
