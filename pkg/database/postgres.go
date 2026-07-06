package database

import (
	"fmt"
	"strings"
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
//
// Before converting, it repairs any primary key / unique index that does not
// include the partitioning column (see repairUniqueIndexes) so that a table
// created by an older model definition cannot leave the server in a
// create_hypertable crash loop.
func EnsureHypertables(db *gorm.DB, tables map[string]string) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error; err != nil {
		return fmt.Errorf("enable timescaledb extension: %w", err)
	}

	for table, timeColumn := range tables {
		if err := repairUniqueIndexes(db, table, timeColumn); err != nil {
			return fmt.Errorf("repair unique indexes for %s: %w", table, err)
		}

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

// repairUniqueIndexes rebuilds every primary key / unique index on table that
// does not include timeColumn, appending the column to its definition.
// TimescaleDB requires all unique indexes on a hypertable to contain the
// partitioning column, and AutoMigrate never alters an existing primary key
// or redefines an existing index — so a table created by an older model
// definition would otherwise make create_hypertable fail on every startup.
// On an up-to-date schema (or a table already converted to a hypertable) the
// detection query finds nothing and this is a no-op.
//
// Expression and partial unique indexes are deliberately skipped: they cannot
// be rebuilt by naively appending a column, and TimescaleDB will report them
// itself if they are incompatible.
func repairUniqueIndexes(db *gorm.DB, table, timeColumn string) error {
	type staleIndex struct {
		IndexName    string
		IsPrimary    bool
		IsConstraint bool
		Columns      string
	}

	var stale []staleIndex
	err := db.Raw(`
		SELECT ix.relname AS index_name,
		       i.indisprimary AS is_primary,
		       EXISTS (
		           SELECT 1 FROM pg_constraint con
		           WHERE con.conindid = i.indexrelid
		       ) AS is_constraint,
		       (
		           SELECT string_agg(quote_ident(a.attname), ', ' ORDER BY k.ord)
		           FROM unnest(i.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord)
		           JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
		       ) AS columns
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_class ix ON ix.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = current_schema()
		  AND t.relname = ?
		  AND i.indisunique
		  AND i.indexprs IS NULL
		  AND i.indpred IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM pg_attribute a
		      WHERE a.attrelid = t.oid
		        AND a.attname = ?
		        AND a.attnum = ANY (i.indkey::int2[])
		  )`, table, timeColumn).Scan(&stale).Error
	if err != nil {
		return fmt.Errorf("detect incompatible unique indexes: %w", err)
	}

	for _, idx := range stale {
		columns := idx.Columns + ", " + quoteIdent(timeColumn)
		err := db.Transaction(func(tx *gorm.DB) error {
			// A primary key (and any unique index backing a constraint) must be
			// dropped via its constraint; a plain unique index via DROP INDEX.
			drop := fmt.Sprintf("DROP INDEX %s", quoteIdent(idx.IndexName))
			if idx.IsConstraint {
				drop = fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", quoteIdent(table), quoteIdent(idx.IndexName))
			}
			if err := tx.Exec(drop).Error; err != nil {
				return err
			}

			recreate := fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", quoteIdent(idx.IndexName), quoteIdent(table), columns)
			if idx.IsPrimary {
				recreate = fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s PRIMARY KEY (%s)", quoteIdent(table), quoteIdent(idx.IndexName), columns)
			}
			return tx.Exec(recreate).Error
		})
		if err != nil {
			return fmt.Errorf("rebuild %s to include %s: %w", idx.IndexName, timeColumn, err)
		}
	}

	return nil
}

// quoteIdent double-quotes a Postgres identifier.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
