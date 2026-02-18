package asagumo

import (
	"database/sql"
	"os"
	"path/filepath"
)

func InitDB() (*sql.DB, error) {
	dbPath := filepath.Join("testdata", "spec.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath)
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
