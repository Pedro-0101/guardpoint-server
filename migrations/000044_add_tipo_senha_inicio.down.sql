ALTER TABLE checkins DROP CONSTRAINT IF EXISTS ck_checkins_tipo_senha;

ALTER TABLE checkins ADD CONSTRAINT ck_checkins_tipo_senha
    CHECK (tipo_senha IS NULL OR tipo_senha IN ('ok', 'emergencia', 'customizada'));
