-- Remove o sistema de niveis de escalonamento.
-- As senhas vigia passam a ter destinatarios e atraso_minutos diretamente.

-- 1. Drop constraints/indexes que referenciam nivel_escalonamento_id
ALTER TABLE senhas_vigia DROP CONSTRAINT IF EXISTS ck_senhas_vigia_nivel_obrigatorio;
ALTER TABLE senhas_vigia DROP CONSTRAINT IF EXISTS senhas_vigia_nivel_escalonamento_id_fkey;
DROP INDEX IF EXISTS uq_senhas_vigia_usuario_nivel;
DROP INDEX IF EXISTS idx_senhas_vigia_nivel;

-- 2. Adicionar novas colunas a senhas_vigia
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS atraso_minutos INTEGER NOT NULL DEFAULT 0;
ALTER TABLE senhas_vigia ADD COLUMN IF NOT EXISTS destinatarios UUID[] NOT NULL DEFAULT '{}';

-- 3. Migrar dados existentes: copiar atraso_minutos e destinatarios do config_escalonamento
UPDATE senhas_vigia sv
SET atraso_minutos = ce.atraso_minutos,
    destinatarios = ARRAY(
        SELECT ced.usuario_id
        FROM config_escalonamento_destinatarios ced
        WHERE ced.config_escalonamento_id = ce.id
    )
FROM config_escalonamento ce
WHERE sv.nivel_escalonamento_id = ce.id;

-- Senhas do tipo ok ficam com atraso 0 e array vazio (nunca disparam alerta)
UPDATE senhas_vigia SET atraso_minutos = 0 WHERE tipo = 'ok' AND atraso_minutos IS DISTINCT FROM 0;

-- 4. Remover coluna antiga
ALTER TABLE senhas_vigia DROP COLUMN IF EXISTS nivel_escalonamento_id;

-- 5. Remover tabelas antigas
DROP TABLE IF EXISTS config_escalonamento_destinatarios;
DROP TABLE IF EXISTS config_escalonamento;
