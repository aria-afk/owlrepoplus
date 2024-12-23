package pg

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

type PG struct {
	Conn     *sql.DB
	QueryMap map[string]string
}

// TODO: Errors
func NewPG() *PG {
	db, err := sql.Open("postgres", os.Getenv("PG_CONN_STRING"))
	if err != nil {
		// NOTE: For both the API and ingestor the DB is critical.
		panic(fmt.Sprintf("Could not open database connection: \n%s", err))
	}
	res := &PG{Conn: db, QueryMap: make(map[string]string, 0)}
	res.LoadQueryMap("../queries")
	return res
}

func (pg *PG) LoadQueryMap(dirPath string) error {
	dirs := []string{dirPath}
	for len(dirs) > 0 {
		dir := pop(&dirs)
		files, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, file := range files {
			path := fmt.Sprintf("%s/%s", dir, file.Name())

			if file.IsDir() {
				dirs = append(dirs, path)
				continue
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			extension := strings.Split(file.Name(), ".")
			// Ensure file type (we may want to remove this)
			if extension[len(extension)-1] != "sql" {
				continue
			}

			pg.QueryMap[dir+"/"+extension[0]] = string(data)
		}
	}
	return nil
}

func (pg *PG) Exec(query string, args ...any) error {
	fpath := "../queries/" + query
	q, ok := pg.QueryMap[fpath]
	if !ok {
		return fmt.Errorf("The query provided does not exist: %s, after formatting path = ./pg/%s", query, fpath)
	}
	_, err := pg.Conn.Exec(q, args...)
	return err
}

func pop(arr *[]string) string {
	l := len(*arr)
	rv := (*arr)[l-1]
	*arr = (*arr)[:l-1]
	return rv
}
