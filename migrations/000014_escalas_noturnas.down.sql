-- Atencao: falha se existirem escalas noturnas (hora_fim < hora_inicio).
ALTER TABLE escalas DROP CONSTRAINT ck_escalas_horas;
ALTER TABLE escalas ADD CONSTRAINT ck_escalas_horas CHECK (hora_fim > hora_inicio);
