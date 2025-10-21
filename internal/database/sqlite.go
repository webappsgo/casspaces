package database

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

type SQLite struct {
    db *sql.DB
}

func NewSQLite(path string) (*SQLite, error) {
    db, err := sql.Open("sqlite3", path+"?_foreign_keys=1")
    if err != nil {
        return nil, err
    }

    if err := db.Ping(); err != nil {
        return nil, err
    }

    return &SQLite{db: db}, nil
}

func (s *SQLite) Query(query string, args ...interface{}) (*sql.Rows, error) {
    return s.db.Query(query, args...)
}

func (s *SQLite) QueryRow(query string, args ...interface{}) *sql.Row {
    return s.db.QueryRow(query, args...)
}

func (s *SQLite) Exec(query string, args ...interface{}) (sql.Result, error) {
    return s.db.Exec(query, args...)
}

func (s *SQLite) Close() error {
    return s.db.Close()
}