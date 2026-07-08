UPDATE config_escalonamento SET nivel = -1 WHERE nivel = 1 AND sistema = true AND descricao = 'atraso sem justificativa';
UPDATE config_escalonamento SET nivel = 1 WHERE nivel = 2 AND sistema = true AND descricao = 'emergencia nao especificada';
UPDATE config_escalonamento SET nivel = 2 WHERE nivel = -1 AND sistema = true;
