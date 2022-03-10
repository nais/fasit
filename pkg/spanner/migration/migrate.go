package migration

import (
	"embed"
	"fmt"

	gspanner "cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/spanner"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(adminClient *database.DatabaseAdminClient, dataClient *gspanner.Client) error {
	instance := spanner.NewDB(*adminClient, *dataClient)
	dbDriver, err := spanner.WithInstance(instance, &spanner.Config{
		DatabaseName: dataClient.DatabaseName(),
	})
	if err != nil {
		return fmt.Errorf("spanner with instance: %w", err)
	}

	sourceDriver, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("iofs new: %w", err)
	}
	defer sourceDriver.Close()

	m, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		"spanner", dbDriver,
	)
	if err != nil {
		return fmt.Errorf("new instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("up: %w", err)
	}

	return nil
}
