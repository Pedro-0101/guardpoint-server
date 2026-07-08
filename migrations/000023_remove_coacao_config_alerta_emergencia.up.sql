DELETE FROM config_alerta_emergencia_destinatarios
WHERE config_alerta_emergencia_id IN (
    SELECT id FROM config_alerta_emergencia WHERE tipo = 'coacao'
);
DELETE FROM config_alerta_emergencia WHERE tipo = 'coacao';

ALTER TABLE config_alerta_emergencia DROP CONSTRAINT config_alerta_emergencia_tipo_check;
ALTER TABLE config_alerta_emergencia ADD CONSTRAINT config_alerta_emergencia_tipo_check
    CHECK (tipo IN ('sabotagem', 'no_show'));
