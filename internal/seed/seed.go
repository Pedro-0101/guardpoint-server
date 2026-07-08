package seed

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

func Run(ctx context.Context, empresaRepo *repository.EmpresaRepository, userRepo *repository.UserRepository, empresaService *service.EmpresaService) error {
	slog.Info("running dev seed")

	empresa, err := empresaRepo.FindByCNPJ(ctx, "00000000000191")
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if empresa == nil {
		empresa = &model.Empresa{
			Nome: "Empresa Demo",
			CNPJ: "00000000000191",
		}
		if err := empresaRepo.Create(ctx, empresa); err != nil {
			return err
		}
		slog.Info("empresa criada", "id", empresa.ID.String())
	}

	existingAdmin, err := userRepo.FindByEmail(ctx, "admin@guardpoint.com")
	if err == nil && existingAdmin != nil {
		slog.Info("admin ja existe, pulando seed", "id", existingAdmin.ID.String())
		return nil
	}

	senhaHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &model.User{
		EmpresaID: empresa.ID,
		Nome:      "Administrador",
		Email:     "admin@guardpoint.com",
		SenhaHash: string(senhaHash),
		Role:      "admin",
	}

	if err := userRepo.Create(ctx, admin); err != nil {
		return err
	}

	slog.Info("admin criado", "id", admin.ID.String(), "email", admin.Email)

	return nil
}
