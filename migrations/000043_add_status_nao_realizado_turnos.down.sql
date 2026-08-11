ALTER TABLE turnos DROP CONSTRAINT IF EXISTS ck_turnos_status;

ALTER TABLE turnos ADD CONSTRAINT ck_turnos_status
    CHECK (status IN ('agendado', 'em_andamento', 'pausado', 'critico', 'finalizado', 'atrasado'));
