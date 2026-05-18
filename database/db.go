package database

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func ConnectDB() *sql.DB {
	// ✅ FIX: tambah TimeZone=Asia/Makassar agar lib/pq membaca timestamp
	// dari PostgreSQL langsung dalam WITA, bukan UTC.
	connStr := "host=localhost user=postgres password=agusadi1 dbname=absensi_karyawanBTW port=5432 sslmode=disable TimeZone=Asia/Makassar"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("DB Open Error:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("DB Connection Error:", err)
	}

	// ✅ FIX: SET timezone di level session sebagai lapisan kedua —
	// memastikan timezone tetap berlaku meski connection di-pool ulang.
	_, err = db.Exec("SET timezone = 'Asia/Makassar'")
	if err != nil {
		log.Fatal("DB Set Timezone Error:", err)
	}

	log.Println("Connected to DB successfully (timezone: Asia/Makassar / WITA)")
	return db
}
