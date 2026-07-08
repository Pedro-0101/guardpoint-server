DELETE FROM config_escalonamento WHERE sistema = true AND nivel = 2;
ALTER TABLE config_escalonamento DROP COLUMN IF EXISTS sistema;
