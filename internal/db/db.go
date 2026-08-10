package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"evaluer/internal/dal"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() (*sql.DB, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr != "" {
		connStr = strings.Replace(connStr, "postgresql+psycopg2://", "postgres://", 1)
		if !strings.Contains(connStr, "sslmode=") {
			if strings.Contains(connStr, "?") {
				connStr += "&sslmode=disable"
			} else {
				connStr += "?sslmode=disable"
			}
		}
	}
	if connStr == "" || strings.HasPrefix(connStr, "postgresql+psycopg2") {
		host := os.Getenv("POSTGRES_HOST")
		if host == "" {
			host = "20.13.2.62"
		}
		port := os.Getenv("POSTGRES_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("POSTGRES_USER")
		if user == "" {
			user = "root"
		}
		pass := os.Getenv("POSTGRES_PASSWORD")
		if pass == "" {
			pass = "63a9f0ea7bb98050796b649e85481845"
		}
		dbname := os.Getenv("POSTGRES_DB")
		if dbname == "" {
			dbname = "core"
		}
		connStr = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&connect_timeout=5", user, pass, host, port, dbname)
	}

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("sql.Open failed: %w", err)
	}

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(5 * time.Minute)

	if err := database.Ping(); err != nil {
		log.Printf("Warning: Database ping failed: %v", err)
	} else {
		log.Println("Database connection pool established successfully")
		dal.EnsureIndexes(database)
	}

	DB = database
	return database, nil
}
