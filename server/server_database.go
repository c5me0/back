package server

import (
	"context"
	"fmt"
	"time"

	"github.com/DeltaLaboratory/contrib/atlasutil"
	"github.com/neko-sc/ent/dialect/pg"

	"cameo/internal/config"
	"cameo/internal/ent"
	"cameo/internal/tools/pgconfig"
)

func (s *Server) setupDatabase(ctx context.Context) error {
	if err := atlasutil.Migrate(ctx, s.config.DB.URI(), config.MigrationsDir(), ""); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	connectionConfig, err := pgconfig.PoolConfig(s.config.DB.URI())
	if err != nil {
		return fmt.Errorf("failed to parse database URI: %w", err)
	}

	connectionConfig.MaxConns = 20
	connectionConfig.MinConns = 2
	connectionConfig.MaxConnLifetime = time.Hour
	connectionConfig.MaxConnIdleTime = time.Minute * 5
	connectionConfig.HealthCheckPeriod = time.Minute

	driver, err := pg.OpenConfig(ctx, connectionConfig)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	s.db = ent.NewClient(ent.Driver(driver))
	s.sqlDB = driver.DB()

	return nil
}
