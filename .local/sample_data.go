package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgres://root:63a9f0ea7bb98050796b649e85481845@20.13.2.62:5432/core?sslmode=disable&connect_timeout=5"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	fmt.Println("--- USERS ---")
	uRows, err := db.Query("SELECT id, username, first_name, last_name, clearance FROM management_courseuser LIMIT 10")
	if err == nil {
		for uRows.Next() {
			var id, clearance int
			var uname, fname, lname string
			uRows.Scan(&id, &uname, &fname, &lname, &clearance)
			fmt.Printf("User ID: %d, Username: %s, Name: %s %s, Clearance: %d\n", id, uname, fname, lname, clearance)
		}
		uRows.Close()
	}

	fmt.Println("\n--- SUBJECTS, MODULES & EXERCISES ---")
	sRows, err := db.Query("SELECT id, name FROM course_subject")
	if err == nil {
		for sRows.Next() {
			var sid int
			var sname string
			sRows.Scan(&sid, &sname)
			fmt.Printf("Subject [%d] %s\n", sid, sname)
			mRows, _ := db.Query("SELECT id, name FROM course_module WHERE parent_subject_id = $1", sid)
			for mRows.Next() {
				var mid int
				var mname string
				mRows.Scan(&mid, &mname)
				fmt.Printf("  Module [%d] %s\n", mid, mname)
				eRows, _ := db.Query("SELECT id, name FROM course_exercise WHERE parent_module_id = $1", mid)
				for eRows.Next() {
					var eid int
					var ename string
					eRows.Scan(&eid, &ename)
					fmt.Printf("    Exercise [%d] %s\n", eid, ename)
				}
				eRows.Close()
			}
			mRows.Close()
		}
		sRows.Close()
	}

	fmt.Println("\n--- SAMPLE ASSIGNMENT RESPONSES & CONTENT ---")
	rRows, err := db.Query(`
		SELECT r.id, r.thread_id, r.user_id, r.date, r.segel_only, r.response_type, c.content 
		FROM assignments_assignmentresponse r
		LEFT JOIN assignments_assignmentresponsecontent c ON c.response_id = r.id
		WHERE r.segel_only = true
		LIMIT 10
	`)
	if err == nil {
		for rRows.Next() {
			var rid, tid, uid int
			var rdate, rtype string
			var segel bool
			var content sql.NullString
			rRows.Scan(&rid, &tid, &uid, &rdate, &segel, &rtype, &content)
			fmt.Printf("Resp ID: %d, Thread: %d, User: %d, Type: %s, Content: %s\n", rid, tid, uid, rtype, content.String)
		}
		rRows.Close()
	}
}
