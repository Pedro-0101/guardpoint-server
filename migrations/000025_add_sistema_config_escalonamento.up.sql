ALTER TABLE config_escalonamento ADD COLUMN sistema BOOLEAN NOT NULL DEFAULT false;

UPDATE config_escalonamento SET sistema = true WHERE nivel = 1;

INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos, sistema)
SELECT e.id, 2, 0, true
FROM empresas e
WHERE NOT EXISTS (
    SELECT 1 FROM config_escalonamento c
    WHERE c.empresa_id = e.id AND c.nivel = 2
);
