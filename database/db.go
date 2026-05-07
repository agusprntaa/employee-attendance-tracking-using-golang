package database

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func ConnectDB() *sql.DB {
	connStr := "host=localhost user=postgres password=agusadi1 dbname=absensi_karyawanBTW port=5432 sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("DB Open Error:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("DB Connection Error:", err)
	}

	log.Println("Connected to DB successfully")
	return db
}
