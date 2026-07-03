ALTER TABLE sessoes_dispositivo DROP COLUMN IF EXISTS device_secret_hash;
ALTER TABLE sessoes_dispositivo DROP CONSTRAINT IF EXISTS uq_sessoes_dispositivo_empresa_device;
