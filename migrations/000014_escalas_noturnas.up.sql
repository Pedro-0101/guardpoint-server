-- Permite escalas noturnas que cruzam a meia-noite (hora_fim < hora_inicio).
-- hora_fim = hora_inicio continua proibido por ser ambiguo (turno de 0h ou 24h).
ALTER TABLE escalas DROP CONSTRAINT ck_escalas_horas;
ALTER TABLE escalas ADD CONSTRAINT ck_escalas_horas CHECK (hora_fim <> hora_inicio);
