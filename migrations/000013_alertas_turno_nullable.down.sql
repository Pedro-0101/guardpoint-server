-- Remove alertas sem turno antes de restaurar a restricao NOT NULL.
DELETE FROM alertas WHERE turno_id IS NULL;
ALTER TABLE alertas ALTER COLUMN turno_id SET NOT NULL;
