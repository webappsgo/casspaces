package database

import (
    "database/sql"
    "fmt"
)

type Database interface {
    Query(query string, args ...interface{}) (*sql.Rows, error)
    QueryRow(query string, args ...interface{}) *sql.Row
    Exec(query string, args ...interface{}) (sql.Result, error)
    Close() error
}

type Config struct {
    Type string
    Path string
}

func New(config *Config) (Database, error) {
    switch config.Type {
    case "sqlite":
        return NewSQLite(config.Path)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", config.Type)
    }
}