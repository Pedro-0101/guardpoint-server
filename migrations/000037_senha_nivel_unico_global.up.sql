-- Um nivel de escalonamento so pode estar vinculado a uma unica senha
-- CUSTOMIZADA em toda a empresa (nao apenas por vigia). Senhas de emergencia
-- ficam de fora dessa regra: todas elas compartilham, por design, o mesmo
-- nivel do sistema (sistema=true).

DROP INDEX IF EXISTS uq_senhas_vigia_usuario_nivel;

CREATE UNIQUE INDEX uq_senhas_vigia_nivel
    ON senhas_vigia (nivel_escalonamento_id)
    WHERE nivel_escalonamento_id IS NOT NULL AND tipo = 'customizada';
