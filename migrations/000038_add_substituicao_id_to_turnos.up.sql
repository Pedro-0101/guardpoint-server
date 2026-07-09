ALTER TABLE turnos
ADD COLUMN substituicao_id UUID REFERENCES substituicoes(id);
