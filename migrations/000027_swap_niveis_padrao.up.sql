UPDATE config_escalonamento SET nivel = -1 WHERE nivel = 1 AND sistema = true;
UPDATE config_escalonamento SET nivel = 1 WHERE nivel = 2 AND sistema = true;
UPDATE config_escalonamento SET nivel = 2 WHERE nivel = -1 AND sistema = true;
