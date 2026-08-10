package dal

import (
	"database/sql"
	"os"
	"strings"
	"time"

	"evaluer/internal/model"
)

type DAL struct {
	db *sql.DB
}

func NewDAL(db *sql.DB) *DAL {
	return &DAL{db: db}
}

// GetIgnoredUsernames reads comma-separated usernames to ignore from env
func (d *DAL) GetIgnoredUsernames() map[string]bool {
	ignored := make(map[string]bool)
	raw := os.Getenv("IGNORED_USERNAMES")
	if raw != "" {
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				ignored[strings.ToLower(trimmed)] = true
			}
		}
	}
	return ignored
}

func (d *DAL) FetchSubjects() ([]model.SubjectEntity, error) {
	rows, err := d.db.Query("SELECT id, name, parent_program_id FROM course_subject ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.SubjectEntity
	for rows.Next() {
		var s model.SubjectEntity
		if err := rows.Scan(&s.ID, &s.Name, &s.ParentProgramID); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func (d *DAL) FetchModules() ([]model.ModuleEntity, error) {
	rows, err := d.db.Query(`SELECT id, name, COALESCE("order", ''), parent_subject_id FROM course_module ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.ModuleEntity
	for rows.Next() {
		var m model.ModuleEntity
		if err := rows.Scan(&m.ID, &m.Name, &m.Order, &m.ParentSubjectID); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}

func (d *DAL) FetchExercises() ([]model.ExerciseEntity, error) {
	rows, err := d.db.Query(`SELECT id, name, COALESCE("order", ''), parent_module_id FROM course_exercise ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.ExerciseEntity
	for rows.Next() {
		var e model.ExerciseEntity
		if err := rows.Scan(&e.ID, &e.Name, &e.Order, &e.ParentModuleID); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

func (d *DAL) FetchStudents(targetUsernames []string) ([]model.UserEntity, error) {
	ignoredMap := d.GetIgnoredUsernames()
	query := `SELECT id, username, COALESCE(first_name, ''), COALESCE(last_name, ''), clearance FROM management_courseuser WHERE clearance = 1`
	
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targetMap := make(map[string]bool)
	for _, u := range targetUsernames {
		if strings.TrimSpace(u) != "" {
			targetMap[strings.ToLower(strings.TrimSpace(u))] = true
		}
	}

	var list []model.UserEntity
	for rows.Next() {
		var u model.UserEntity
		if err := rows.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &u.Clearance); err != nil {
			return nil, err
		}
		lowerUname := strings.ToLower(u.Username)
		if ignoredMap[lowerUname] {
			continue
		}
		if len(targetMap) > 0 && !targetMap[lowerUname] {
			continue
		}
		list = append(list, u)
	}
	return list, nil
}

func (d *DAL) FetchAssignments() ([]model.AssignmentEntity, error) {
	query := `
		SELECT id, date, exercise_id, user_id, 
		       COALESCE(last_staff_updated, date), 
		       COALESCE(last_updated, date), 
		       assignment_status 
		FROM assignments_assignment
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.AssignmentEntity
	for rows.Next() {
		var a model.AssignmentEntity
		if err := rows.Scan(&a.ID, &a.Date, &a.ExerciseID, &a.UserID, &a.LastStaffUpdated, &a.LastUpdated, &a.AssignmentStatus); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

func (d *DAL) FetchResponses() ([]model.ResponseEntity, error) {
	query := `
		SELECT r.id, r.date, r.segel_only, COALESCE(r.response_type, ''), r.thread_id, r.user_id, COALESCE(c.content, '')
		FROM assignments_assignmentresponse r
		LEFT JOIN assignments_assignmentresponsecontent c ON c.response_id = r.id
		ORDER BY r.date ASC
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.ResponseEntity
	for rows.Next() {
		var r model.ResponseEntity
		var rdate time.Time
		if err := rows.Scan(&r.ID, &rdate, &r.SegelOnly, &r.ResponseType, &r.ThreadID, &r.UserID, &r.Content); err != nil {
			return nil, err
		}
		r.Date = rdate
		list = append(list, r)
	}
	return list, nil
}
