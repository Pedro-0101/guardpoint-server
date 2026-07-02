ALTER TABLE checkins ADD COLUMN IF NOT EXISTS cliente_checkin_id UUID;

ALTER TABLE checkins ADD CONSTRAINT uq_checkins_cliente UNIQUE (turno_id, cliente_checkin_id);
