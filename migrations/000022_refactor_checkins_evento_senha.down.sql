DROP INDEX IF EXISTS idx_checkins_evento;

ALTER TABLE checkins DROP COLUMN IF EXISTS senha_vigia_id;

ALTER TABLE checkins DROP CONSTRAINT IF EXISTS ck_checkins_tipo_senha;

-- Remapeia o vocabulario novo de volta para o antigo (ok->padrao, emergencia->coacao).
UPDATE checkins SET tipo_senha = 'padrao' WHERE tipo_senha = 'ok';
UPDATE checkins SET tipo_senha = 'coacao' WHERE tipo_senha = 'emergencia';

-- As linhas com evento IN ('finalizacao','sabotagem') ficaram com tipo_senha
-- IS NULL no up.sql; reconstroi o valor antigo a partir de evento.
UPDATE checkins SET tipo_senha = evento WHERE evento IN ('finalizacao', 'sabotagem') AND tipo_senha IS NULL;

-- Qualquer linha remanescente sem tipo_senha (ex: evento = 'inicio', que nao existia
-- no vocabulario antigo) recebe 'padrao' como valor mais proximo, para satisfazer o NOT NULL.
UPDATE checkins SET tipo_senha = 'padrao' WHERE tipo_senha IS NULL;

ALTER TABLE checkins ALTER COLUMN tipo_senha SET NOT NULL;

ALTER TABLE checkins DROP CONSTRAINT IF EXISTS ck_checkins_evento;

ALTER TABLE checkins DROP COLUMN IF EXISTS evento;
