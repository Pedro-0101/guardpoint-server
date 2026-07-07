-- migrations/000018_config_alerta_destinatarios.down.sql
DROP TABLE IF EXISTS config_alerta_emergencia_destinatarios;
DROP TABLE IF EXISTS config_alerta_emergencia;
DROP TABLE IF EXISTS config_escalonamento_destinatarios;

ALTER TABLE config_escalonamento ADD COLUMN whatsapp_para VARCHAR(20);
ALTER TABLE config_escalonamento ADD COLUMN cargo_alvo VARCHAR(50);
