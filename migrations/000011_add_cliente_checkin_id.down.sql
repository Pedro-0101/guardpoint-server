ALTER TABLE checkins DROP CONSTRAINT IF EXISTS uq_checkins_cliente;

ALTER TABLE checkins DROP COLUMN IF EXISTS cliente_checkin_id;
