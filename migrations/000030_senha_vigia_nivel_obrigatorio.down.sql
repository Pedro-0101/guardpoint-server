DROP INDEX IF EXISTS uq_senhas_vigia_usuario_nivel;

ALTER TABLE senhas_vigia DROP CONSTRAINT IF EXISTS ck_senhas_vigia_nivel_obrigatorio;

ALTER TABLE senhas_vigia ADD CONSTRAINT ck_senhas_vigia_nivel_fixo_so_customizada
    CHECK (tipo = 'customizada' OR nivel_escalonamento_id IS NULL);
