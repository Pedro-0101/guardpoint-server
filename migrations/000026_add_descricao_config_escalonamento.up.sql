ALTER TABLE config_escalonamento ADD COLUMN descricao VARCHAR(255) NOT NULL DEFAULT '';

UPDATE config_escalonamento SET descricao = 'atraso sem justificativa' WHERE nivel = 1 AND sistema = true;
UPDATE config_escalonamento SET descricao = 'emergencia nao especificada' WHERE nivel = 2 AND sistema = true;
