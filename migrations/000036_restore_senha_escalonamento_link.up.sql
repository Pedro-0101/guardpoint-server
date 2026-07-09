-- Restaura o vinculo entre senhas_vigia e config_escalonamento
-- (removido anteriormente pela migration 000033).
-- Remove as tabelas config_alerta_emergencia* que deixam de ser usadas:
-- a resolucao de destinatarios para alertas de senha passa a usar
-- o nivel_escalonamento_id diretamente; para sabotagem/no_show,
-- todos os destinatarios de todos os escalonamentos da empresa.

ALTER TABLE senhas_vigia
    ADD COLUMN nivel_escalonamento_id UUID REFERENCES config_escalonamento(id) ON DELETE RESTRICT;

-- Backfill: senhas de emergencia/customizada existentes recebem o nivel de
-- escalonamento padrao do sistema (sistema=true) da propria empresa, para
-- nao violar a constraint abaixo.
UPDATE senhas_vigia sv
SET nivel_escalonamento_id = ce.id
FROM config_escalonamento ce
WHERE sv.tipo != 'ok'
  AND sv.nivel_escalonamento_id IS NULL
  AND ce.empresa_id = sv.empresa_id
  AND ce.sistema = true;

ALTER TABLE senhas_vigia
    ADD CONSTRAINT ck_senhas_vigia_nivel_obrigatorio
        CHECK (tipo = 'ok' OR nivel_escalonamento_id IS NOT NULL);

CREATE UNIQUE INDEX uq_senhas_vigia_usuario_nivel
    ON senhas_vigia (usuario_id, nivel_escalonamento_id)
    WHERE nivel_escalonamento_id IS NOT NULL;

DROP TABLE IF EXISTS config_alerta_emergencia_destinatarios;
DROP TABLE IF EXISTS config_alerta_emergencia;
