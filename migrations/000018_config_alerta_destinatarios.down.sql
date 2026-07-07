-- migrations/000018_config_alerta_destinatarios.down.sql
DROP TABLE IF EXISTS config_alerta_emergencia_destinatarios;
DROP TABLE IF EXISTS config_alerta_emergencia;
DROP TABLE IF EXISTS config_escalonamento_destinatarios;

-- Recria apenas o formato das colunas antigas (nullable), sem restaurar dados:
-- nao ha como saber o whatsapp_para/cargo_alvo original apos o DROP COLUMN do
-- up.sql. A coluna original era NOT NULL; aqui fica nullable de proposito, ja
-- que nao teriamos como popular um valor obrigatorio para linhas existentes.
ALTER TABLE config_escalonamento ADD COLUMN whatsapp_para VARCHAR(20);
ALTER TABLE config_escalonamento ADD COLUMN cargo_alvo VARCHAR(50);
