-- Reverte apenas o CHECK constraint para voltar a aceitar 'coacao'. Nao recria
-- as linhas de config_alerta_emergencia/config_alerta_emergencia_destinatarios
-- deletadas no up.sql: nao ha como saber quais destinatarios estavam associados
-- ao tipo 'coacao' apos o DELETE, entao os dados sao perdidos de forma
-- irreversivel por este rollback.
ALTER TABLE config_alerta_emergencia DROP CONSTRAINT config_alerta_emergencia_tipo_check;
ALTER TABLE config_alerta_emergencia ADD CONSTRAINT config_alerta_emergencia_tipo_check
    CHECK (tipo IN ('coacao', 'sabotagem', 'no_show'));
