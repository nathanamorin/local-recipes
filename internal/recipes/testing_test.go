package recipes

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestDB returns an in-memory SQLite database with this package's
// schema migrated, for fast unit tests without a running Postgres
// instance. Production uses Postgres (see InitDB) - this is a
// close-enough stand-in since the package avoids Postgres-only SQL
// outside of recipePoolLeastRecentlyCooked's query, which is exercised by
// TestGenerateWeek below and is plain ANSI SQL.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Each test gets its own uniquely-named in-memory database: SQLite's
	// shared-cache mode makes every connection opened with the same
	// "file:<name>?mode=memory&cache=shared" DSN see the same in-memory
	// database (needed so GORM's connection pool - multiple *sql.DB
	// connections - all see the same data within one test), but reusing
	// one fixed name across tests would leak every test's rows into
	// every other test's "isolated" database.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %s", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("automigrate: %s", err)
	}
	return db
}
