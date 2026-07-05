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
