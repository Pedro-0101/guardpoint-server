ALTER TABLE checkins ADD COLUMN evento VARCHAR(20);

UPDATE checkins SET evento = 'finalizacao' WHERE tipo_senha = 'finalizacao';
UPDATE checkins SET evento = 'sabotagem'   WHERE tipo_senha = 'sabotagem';
UPDATE checkins SET evento = 'checkin'     WHERE evento IS NULL;

ALTER TABLE checkins ALTER COLUMN evento SET NOT NULL;
ALTER TABLE checkins ADD CONSTRAINT ck_checkins_evento
    CHECK (evento IN ('inicio', 'checkin', 'finalizacao', 'sabotagem'));

-- Remapeia o vocabulario antigo para o novo (padrao->ok, coacao->emergencia);
-- finalizacao/sabotagem eram marcadores de acao, nao de senha, viram NULL.
-- DROP NOT NULL precisa vir antes do UPDATE que zera tipo_senha para essas
-- linhas, senao a constraint NOT NULL ainda vigente rejeita o UPDATE.
ALTER TABLE checkins ALTER COLUMN tipo_senha DROP NOT NULL;

UPDATE checkins SET tipo_senha = 'ok'         WHERE tipo_senha = 'padrao';
UPDATE checkins SET tipo_senha = 'emergencia' WHERE tipo_senha = 'coacao';
UPDATE checkins SET tipo_senha = NULL         WHERE tipo_senha IN ('finalizacao', 'sabotagem');

ALTER TABLE checkins ADD CONSTRAINT ck_checkins_tipo_senha
    CHECK (tipo_senha IS NULL OR tipo_senha IN ('ok', 'emergencia', 'customizada'));

ALTER TABLE checkins ADD COLUMN senha_vigia_id UUID REFERENCES senhas_vigia(id) ON DELETE SET NULL;

CREATE INDEX idx_checkins_evento ON checkins(evento);
