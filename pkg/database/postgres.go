package database

import (
	"fmt"
	"time"

	"PawonWarga-BE/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	// password is appended separately, and only when non-empty: an empty
	// unquoted "password=" token in a space-separated DSN can make pgx's
	// parser swallow the fields after it (dbname, sslmode, ...), silently
	// dropping dbname and causing Postgres to fall back to using the
	// username as the database name.
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		cfg.Host, cfg.Port, cfg.User, cfg.Name, cfg.SSLMode,
	)
	if cfg.Password != "" {
		dsn += fmt.Sprintf(" password=%s", cfg.Password)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		// TimescaleDB does not support foreign keys that reference a hypertable
		// (see model.Post/model.Comment), so FK constraint creation is disabled
		// globally rather than per-model. Preload/association queries still work —
		// GORM only needs the foreignKey tag for those, not a DB-level constraint.
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

// EnsureHypertables enables the TimescaleDB extension and converts the given
// time-series tables into hypertables partitioned on timeColumn. It is safe
// to call on every startup — if_not_exists makes both operations idempotent.
// Must run after AutoMigrate has created the plain tables.
func EnsureHypertables(db *gorm.DB, tables map[string]string) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error; err != nil {
		return fmt.Errorf("enable timescaledb extension: %w", err)
	}

	for table, timeColumn := range tables {
		// Classic create_hypertable(relation, time_column_name) signature — works
		// across TimescaleDB versions, unlike the newer by_range() dimension builder.
		sql := fmt.Sprintf(
			"SELECT create_hypertable('%s', '%s', if_not_exists => TRUE, migrate_data => TRUE)",
			table, timeColumn,
		)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("create hypertable for %s: %w", table, err)
		}
	}

	return nil
}
