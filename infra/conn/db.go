package conn

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

// ConnectDatabase is creating a new connection to our database
func (db *DB) ConnectDatabase() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")
	dbZone := os.Getenv("DB_ZONE")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=%s", dbHost, dbPort, dbUser, dbPass, dbName, dbZone)

	log.Println("connStr", connStr)

	var err error
	var database *sql.DB

	for attempts := 1; attempts <= 5; attempts++ {
		database, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("Attempt %d: Failed to open DB connection: %v", attempts, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Veritabanı ayarlarını yapılandır
		database.SetMaxOpenConns(25)
		database.SetMaxIdleConns(5)
		database.SetConnMaxLifetime(5 * time.Minute)
		database.SetConnMaxIdleTime(2 * time.Minute)

		// Bağlantıyı test et
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = database.PingContext(ctx)
		cancel()

		if err == nil {
			log.Println("DB Connected successfully")
			db.DB = database
			return
		}

		log.Printf("Attempt %d: Failed to ping DB: %v", attempts, err)
		database.Close()
		time.Sleep(2 * time.Second)
	}

	log.Fatal("Failed to connect to DB after 5 attempts")
}

// CloseDatabase method is closing a connection between your app and your db
func (db *DB) CloseDatabase() {
	if err := db.DB.Close(); err != nil {
		log.Println("Failed to close connection from the database:", err.Error())
	} else {
		log.Println("DB Connection Closed")
	}
}
