-- Alertas de no-show nao possuem turno associado (o vigia nao iniciou nenhum turno).
-- Permite turno_id nulo para que esses alertas possam ser registrados.
ALTER TABLE alertas ALTER COLUMN turno_id DROP NOT NULL;
