-- Vigias passam a se autenticar por codigo da empresa + nome + senha e nao
-- precisam mais de email cadastrado. Admin/supervisor continuam obrigados a
-- ter email e autenticam exclusivamente por email + senha. Como o login de
-- vigia informa o codigo da empresa, o nome so precisa ser unico dentro da
-- mesma empresa (o mesmo nome pode se repetir entre empresas diferentes).

ALTER TABLE usuarios ALTER COLUMN email DROP NOT NULL;

ALTER TABLE usuarios
    ADD CONSTRAINT chk_usuarios_email_obrigatorio_nao_vigia
    CHECK (role = 'vigia' OR email IS NOT NULL);

ALTER TABLE usuarios ADD CONSTRAINT usuarios_empresa_nome_key UNIQUE (empresa_id, nome);
