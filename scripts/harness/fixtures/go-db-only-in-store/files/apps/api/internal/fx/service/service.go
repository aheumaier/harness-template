// Package service violates go-db-only-in-store.
package service

import "database/sql"

// Ping talks to the database from the service layer.
func Ping(db *sql.DB) error { return db.Ping() }
