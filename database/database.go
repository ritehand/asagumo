package database

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func InitDB() (*sql.DB, error) {
	var db *sql.DB
	var err error
	if os.Getenv("KOYEB") != "" {
		koyebPostgres := os.Getenv("KOYEB_POSTGRES")
		if koyebPostgres == "" {
			return nil, errors.New("KOYEB_POSTGRES is not set")
		}
		db, err = sql.Open("postgres", koyebPostgres)

	} else {
		dbPath := filepath.Join("testdata", "spec.db")
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, err
		}

		db, err = sql.Open("sqlite3", dbPath)

	}
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS prefectures (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		district_count INTEGER NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
