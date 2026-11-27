package recipes

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DefaultMaxOpenConns is the recommended maxOpenConns for InitDB when the
// caller has no specific reason to override it.
const DefaultMaxOpenConns = 20

// InitDB opens the single shared PostgreSQL connection pool used by both
// the public API server and the MCP server. dsn is a standard PostgreSQL
// connection string, e.g.
// "postgres://user:password@host:5432/dbname?sslmode=disable".
func InitDB(dsn string, maxOpenConns int) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if maxOpenConns < 1 {
		maxOpenConns = 1
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}

	return db, nil
}
