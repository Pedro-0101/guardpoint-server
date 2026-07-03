-- Remove duplicatas mantendo a linha mais recente por (empresa_id, device_id)
DELETE FROM sessoes_dispositivo s
USING sessoes_dispositivo d
WHERE s.empresa_id = d.empresa_id
  AND s.device_id = d.device_id
  AND (s.criado_em < d.criado_em OR (s.criado_em = d.criado_em AND s.id < d.id));

ALTER TABLE sessoes_dispositivo
ADD CONSTRAINT uq_sessoes_dispositivo_empresa_device UNIQUE (empresa_id, device_id);

-- Hash SHA-256 (hex) do device_secret entregue uma unica vez no registro
-- biometrico. Sessoes antigas ficam NULL e exigem novo registro.
ALTER TABLE sessoes_dispositivo
ADD COLUMN IF NOT EXISTS device_secret_hash CHAR(64);
