DROP INDEX IF EXISTS uq_senhas_vigia_nivel;

CREATE UNIQUE INDEX uq_senhas_vigia_usuario_nivel
    ON senhas_vigia (usuario_id, nivel_escalonamento_id)
    WHERE nivel_escalonamento_id IS NOT NULL;
