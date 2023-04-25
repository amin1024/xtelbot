package repo

import (
	"database/sql"
	"embed"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

var db *sql.DB

//go:embed migrations/*.sql
var embedMigrations embed.FS

func AutoMigrate() {
	if db == nil {
		panic("must SetupDb first")
	}
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}
}

func SetupDb(dsn string) {
	if db != nil {
		return
	}
	// TODO: get the dsn from config
	d, err := sql.Open("sqlite3", dsn)
	if err != nil {
		panic(err)
	}
	db = d
}
