package db

import (
	"database/sql"
	"fmt"
	"net"

	_ "github.com/Kount/pq-timeouts"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type RetriableError struct {
	Inner error
	Msg   string
}

func (r RetriableError) Error() string {
	return fmt.Sprintf("%s: %s", r.Msg, r.Inner.Error())
}

func GetConnectionPool(dbConfig Config) (*sqlx.DB, error) {
	driver := dbConfig.Type
	if driver == "postgres" {
		driver = "pq-timeouts"
	}

	nativeDBConn, err := sql.Open(driver, dbConfig.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %s", err)
	}

	dbConn := sqlx.NewDb(nativeDBConn, dbConfig.Type)

	if err = dbConn.Ping(); err != nil {
		dbConn.Close()
		if netErr, ok := err.(*net.OpError); ok {
			return nil, RetriableError{
				Inner: netErr,
				Msg:   "unable to ping",
			}
		}
		return nil, fmt.Errorf("unable to ping: %s", err)
	}

	return dbConn, nil
}
