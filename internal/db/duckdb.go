package db

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/marcboeker/go-duckdb"
)

var (
	dbInstance *sql.DB
	dbOnce     sync.Once
	dbErr      error
)

func GetDB() (*sql.DB, error) {
	dbOnce.Do(func() {
		dbInstance, dbErr = initializeDuckDB()
	})
	return dbInstance, dbErr
}

func initializeDuckDB() (*sql.DB, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("INSTALL json"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to install JSON extension: %w", err)
	}

	if _, err := db.Exec("LOAD json"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load JSON extension: %w", err)
	}

	return db, nil
}
