package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type UrlDB struct {
	Db *sql.DB
}

// Database configuration
const (
	DB_USER     = "user"
	DB_PASSWORD = "pass"
	DB_NAME     = "dbname"
)

// Global DB error checking
func checkDBErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func (udb *UrlDB) Open() error {
	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", DB_USER, DB_PASSWORD, DB_NAME)
	var err error
	udb.Db, err = sql.Open("postgres", dbinfo)
	if err != nil {
		panic(err)
	}
	// Bootstrap database table
	_, err = udb.Db.Query(`CREATE TABLE IF NOT EXISTS locations (
								id SERIAL PRIMARY KEY,
								location GEOGRAPHY(POINT,4326),
								ip VARCHAR(39),
								name VARCHAR(100),
								photo_ext VARCHAR(5),
								flagged INTEGER,
								rating INTEGER,
								sighting INTEGER,
								last_updated TIMESTAMP,
								t_stamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
							);`)
	checkDBErr(err)
	return err
}
