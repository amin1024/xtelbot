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
	SetupDb()

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}
}

func SetupDb() {
	if db != nil {
		return
	}
	d, err := sql.Open("sqlite3", "./db.db")
	if err != nil {
		panic(err)
	}
	db = d
}
