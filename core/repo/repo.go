package repo

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

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
