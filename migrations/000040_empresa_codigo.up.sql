-- Codigo curto e unico usado pelo vigia para se identificar no login
-- (codigo_empresa + nome + senha), ja que o nome deixou de ser globalmente
-- unico. Empresas novas recebem o codigo gerado pela aplicacao; as linhas
-- existentes sao preenchidas aqui a partir do proprio id.

ALTER TABLE empresas ADD COLUMN codigo VARCHAR(8);

UPDATE empresas SET codigo = upper(substr(replace(id::text, '-', ''), 1, 8)) WHERE codigo IS NULL;

ALTER TABLE empresas ALTER COLUMN codigo SET NOT NULL;

ALTER TABLE empresas ADD CONSTRAINT empresas_codigo_key UNIQUE (codigo);
