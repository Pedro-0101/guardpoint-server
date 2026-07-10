ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS usuarios_empresa_nome_key;
ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS chk_usuarios_email_obrigatorio_nao_vigia;
ALTER TABLE usuarios ALTER COLUMN email SET NOT NULL;
