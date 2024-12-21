package pg

import (
	"database/sql"
	"os"

	_ "github.com/lib/pq"
)

type PG struct {
	conn *sql.DB
}

// TODO: Errors
func NewPG() (*PG, error) {
	db, err := sql.Open("postgres", os.Getenv("PG_CONN_STRING"))
	if err != nil {
		return &PG{}, err
	}
	return &PG{conn: db}, nil
}
