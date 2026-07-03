//go:build integration

// Package testutil prove um Postgres de teste com as migrations aplicadas.
// Compilado apenas com a build tag `integration`.
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultTestDatabaseURL aponta para o container de teste (porta 5433 para nao
// conflitar com o Postgres de desenvolvimento). Sobrescreva com TEST_DATABASE_URL.
const DefaultTestDatabaseURL = "postgres://postgres:postgres@localhost:5433/guardpoint_test?sslmode=disable"

// SetupTestDB conecta no banco de teste, recria o schema public e aplica todas
// as migrations *.up.sql. O pool e fechado no cleanup do teste.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = DefaultTestDatabaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("conectar no banco de teste (%s): %v", url, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("banco de teste indisponivel (%s): %v — suba com `make test-db-up`", url, err)
	}
	t.Cleanup(pool.Close)

	execSimple(t, pool, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")

	for _, file := range migrationFiles(t) {
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ler migration %s: %v", file, err)
		}
		execSimple(t, pool, string(sql))
	}

	return pool
}

// execSimple executa SQL possivelmente multi-statement usando o protocolo
// simples (o protocolo estendido do pgx nao aceita mais de um comando).
func execSimple(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("adquirir conexao: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Conn().PgConn().Exec(ctx, sql).ReadAll(); err != nil {
		t.Fatalf("executar SQL de setup: %v\n%s", err, sql)
	}
}

func migrationFiles(t *testing.T) []string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller falhou")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("nenhuma migration encontrada em %s: %v", dir, err)
	}
	sort.Strings(files)
	return files
}
