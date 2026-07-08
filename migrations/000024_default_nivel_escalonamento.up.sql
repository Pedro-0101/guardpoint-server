INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos)
SELECT id, 1, 15 FROM empresas e
WHERE NOT EXISTS (SELECT 1 FROM config_escalonamento c WHERE c.empresa_id = e.id);

INSERT INTO config_escalonamento_destinatarios (config_escalonamento_id, usuario_id)
SELECT ce.id, u.id
FROM config_escalonamento ce
JOIN usuarios u ON u.empresa_id = ce.empresa_id AND u.role = 'admin'
WHERE NOT EXISTS (SELECT 1 FROM config_escalonamento_destinatarios d WHERE d.config_escalonamento_id = ce.id);
