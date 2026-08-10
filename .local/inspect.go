package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://root:63a9f0ea7bb98050796b649e85481845@20.13.2.62:5432/core?sslmode=disable&connect_timeout=5"
	}

	fmt.Println("Attempting database connection to 20.13.2.62:5432...")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open connection: %v", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(5 * time.Second)
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("Successfully connected to PostgreSQL database core!")

	tables := []string{
		"course_subject",
		"course_module",
		"course_exercise",
		"management_courseuser",
		"assignments_assignment",
		"assignments_assignmentresponse",
		"assignments_assignmentresponsecontent",
	}

	for _, tbl := range tables {
		fmt.Printf("\n--- TABLE: %s ---\n", tbl)
		rows, err := db.Query(`
			SELECT column_name, data_type, is_nullable 
			FROM information_schema.columns 
			WHERE table_name = $1 
			ORDER BY ordinal_position
		`, tbl)
		if err != nil {
			fmt.Printf("Error querying columns for %s: %v\n", tbl, err)
			continue
		}

		for rows.Next() {
			var colName, dataType, isNullable string
			if err := rows.Scan(&colName, &dataType, &isNullable); err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}
			fmt.Printf("  - %-35s %-20s (Nullable: %s)\n", colName, dataType, isNullable)
		}
		rows.Close()

		var count int
		if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tbl)).Scan(&count); err == nil {
			fmt.Printf("  Total count: %d rows\n", count)
		}
	}
}
