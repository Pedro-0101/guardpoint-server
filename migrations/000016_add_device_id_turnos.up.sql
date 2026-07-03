-- Dispositivo que iniciou (ou reassociou via PIN) o turno; check-ins devem vir
-- dele. Turnos anteriores a esta migration ficam NULL e nao sao validados.
ALTER TABLE turnos ADD COLUMN IF NOT EXISTS device_id VARCHAR(255);
