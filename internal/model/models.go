package model

import "time"

// Clearance Levels
const (
	ClearanceHanich  = 1 // Student / Participant
	ClearanceChecker = 2 // Exercise Checker
	ClearanceSegel   = 3 // Staff Member
	ClearanceAdmin   = 5 // Administrator
)

// Assignment Status Integer Mapping
const (
	AssignmentStatusNew            = 0
	AssignmentStatusWorkInProgress = 1
	AssignmentStatusRedo           = 2
	AssignmentStatusSubmitted      = 3
	AssignmentStatusAutoChecked    = 4
	AssignmentStatusDone           = 5
)

// String Representation of Assignment Statuses
const (
	StatusNew            = "New"
	StatusWorkInProgress = "Work In Progress"
	StatusRedo           = "Redo"
	StatusSubmitted      = "Submitted"
	StatusAutoChecked    = "AutoChecked"
	StatusDone           = "Done"
	StatusRedoUnchecked  = "REDO_UNCHECKED"
	StatusWipUnchecked   = "WIP_UNCHECKED"
)

// MapStatusToString converts integer status from Postgres to status string
func MapStatusToString(status int) string {
	switch status {
	case AssignmentStatusNew:
		return StatusNew
	case AssignmentStatusWorkInProgress:
		return StatusWorkInProgress
	case AssignmentStatusRedo:
		return StatusRedo
	case AssignmentStatusSubmitted:
		return StatusSubmitted
	case AssignmentStatusAutoChecked:
		return StatusAutoChecked
	case AssignmentStatusDone:
		return StatusDone
	default:
		return StatusNew
	}
}

// Database Entities

type SubjectEntity struct {
	ID              int    `db:"id" json:"id"`
	Name            string `db:"name" json:"name"`
	Symbol          string `db:"symbol" json:"symbol"`
	ParentProgramID int    `db:"parent_program_id" json:"parent_program_id"`
}

type ModuleEntity struct {
	ID              int    `db:"id" json:"id"`
	Name            string `db:"name" json:"name"`
	Order           string `db:"order" json:"order"`
	ParentSubjectID int    `db:"parent_subject_id" json:"parent_subject_id"`
}

type ExerciseEntity struct {
	ID             int    `db:"id" json:"id"`
	Name           string `db:"name" json:"name"`
	Order          string `db:"order" json:"order"`
	ParentModuleID int    `db:"parent_module_id" json:"parent_module_id"`
}

type UserEntity struct {
	ID        int    `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	FirstName string `db:"first_name" json:"first_name"`
	LastName  string `db:"last_name" json:"last_name"`
	Clearance int    `db:"clearance" json:"clearance"`
}

type StudentResponse struct {
	StudentID int    `json:"student_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type AssignmentEntity struct {
	ID               int       `db:"id" json:"id"`
	Date             time.Time `db:"date" json:"date"`
	ExerciseID       int       `db:"exercise_id" json:"exercise_id"`
	UserID           int       `db:"user_id" json:"user_id"`
	LastStaffUpdated time.Time `db:"last_staff_updated" json:"last_staff_updated"`
	LastUpdated      time.Time `db:"last_updated" json:"last_updated"`
	AssignmentStatus int       `db:"assignment_status" json:"assignment_status"`
}

type ResponseEntity struct {
	ID           int       `db:"id" json:"id"`
	Date         time.Time `db:"date" json:"date"`
	SegelOnly    bool      `db:"segel_only" json:"segel_only"`
	ResponseType string    `db:"response_type" json:"response_type"`
	ThreadID     int       `db:"thread_id" json:"thread_id"`
	UserID       int       `db:"user_id" json:"user_id"`
	Content      string    `db:"content" json:"content"`
}

// Config Models

type ExerciseConfig struct {
	Name  string  `yaml:"name" json:"name"`
	Score float64 `yaml:"score" json:"score"`
}

type ModuleConfig struct {
	Name      string                    `yaml:"name" json:"name"`
	Weight    float64                   `yaml:"weight" json:"weight"`
	Exercises map[string]ExerciseConfig `yaml:"exercises" json:"exercises"`
}

type SubjectConfig struct {
	Name    string                  `yaml:"name" json:"name"`
	Weight  float64                 `yaml:"weight" json:"weight"`
	Modules map[string]ModuleConfig `yaml:"modules" json:"modules"`
}

type CourseConfig struct {
	Subjects map[string]SubjectConfig `yaml:"subjects" json:"subjects"`
}

// Request & Response Payload DTOs

type ExerciseCalcRequest struct {
	SubjectName  string   `json:"subject_name"`
	ModuleName   string   `json:"module_name"`
	ExerciseName string   `json:"exercise_name"`
	Students     []string `json:"students"`
}

type ExerciseGradeResponse struct {
	StudentID     int     `json:"student_id"`
	Username      string  `json:"username"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	RawGrade      float64 `json:"raw_grade"`
	Score         float64 `json:"score"`
	Review        string  `json:"review"`
	Status        string  `json:"status"`
	AssignmentURL string  `json:"assignment_url"`
	RedoCount     int     `json:"redo_count"`
	IsChecked     bool    `json:"is_checked"`
}

type ExerciseCalcResponse struct {
	SubjectName  string                  `json:"subject_name"`
	ModuleName   string                  `json:"module_name"`
	ExerciseName string                  `json:"exercise_name"`
	Grades       []ExerciseGradeResponse `json:"grades"`
}

type ModuleCalcRequest struct {
	SubjectName string   `json:"subject_name"`
	ModuleName  string   `json:"module_name"`
	Students    []string `json:"students"`
}

type ExerciseGradeInModule struct {
	ExerciseName  string  `json:"exercise_name"`
	RawGrade      float64 `json:"raw_grade"`
	Score         float64 `json:"score"`
	Review        string  `json:"review"`
	Status        string  `json:"status"`
	AssignmentURL string  `json:"assignment_url"`
	RedoCount     int     `json:"redo_count"`
	IsChecked     bool    `json:"is_checked"`
}

type StudentModuleGrade struct {
	StudentID   int                     `json:"student_id"`
	Username    string                  `json:"username"`
	FirstName   string                  `json:"first_name"`
	LastName    string                  `json:"last_name"`
	ModuleGrade float64                 `json:"module_grade"`
	Exercises   []ExerciseGradeInModule `json:"exercises"`
}

type ModuleCalcResponse struct {
	SubjectName string               `json:"subject_name"`
	ModuleName  string               `json:"module_name"`
	Students    []StudentModuleGrade `json:"students"`
}

type SubjectCalcRequest struct {
	SubjectName string   `json:"subject_name"`
	Students    []string `json:"students"`
}

type ModuleGradeInSubject struct {
	ModuleName  string  `json:"module_name"`
	ModuleGrade float64 `json:"module_grade"`
}

type StudentSubjectGrade struct {
	StudentID    int                     `json:"student_id"`
	Username     string                  `json:"username"`
	FirstName    string                  `json:"first_name"`
	LastName     string                  `json:"last_name"`
	SubjectGrade float64                 `json:"subject_grade"`
	Modules      []ModuleGradeInSubject `json:"modules"`
}

type SubjectCalcResponse struct {
	SubjectName string                `json:"subject_name"`
	Students    []StudentSubjectGrade `json:"students"`
}

type CourseCalcRequest struct {
	Students []string `json:"students"`
}

type SubjectGradeInCourse struct {
	SubjectName  string  `json:"subject_name"`
	SubjectGrade float64 `json:"subject_grade"`
}

type StudentCourseGrade struct {
	StudentID  int                    `json:"student_id"`
	Username   string                 `json:"username"`
	FirstName  string                 `json:"first_name"`
	LastName   string                 `json:"last_name"`
	TotalGrade float64                `json:"total_grade"`
	Subjects   []SubjectGradeInCourse `json:"subjects"`
}

type CourseCalcResponse struct {
	Students []StudentCourseGrade `json:"students"`
}

type StudentDetailRequest struct {
	Username string `json:"username"`
}

type StudentDetailModuleGrade struct {
	ModuleName  string                  `json:"module_name"`
	ModuleGrade float64                 `json:"module_grade"`
	Exercises   []ExerciseGradeInModule `json:"exercises"`
}

type StudentDetailSubjectGrade struct {
	SubjectName  string                     `json:"subject_name"`
	SubjectGrade float64                    `json:"subject_grade"`
	Modules      []StudentDetailModuleGrade `json:"modules"`
}

type StudentDetailResponse struct {
	StudentID  int                         `json:"student_id"`
	Username   string                      `json:"username"`
	FirstName  string                      `json:"first_name"`
	LastName   string                      `json:"last_name"`
	TotalGrade float64                     `json:"total_grade"`
	Subjects   []StudentDetailSubjectGrade `json:"subjects"`
}

type StudentRecRequest struct {
	Username    string  `json:"username"`
	TargetScore float64 `json:"target_score"`
}

type RecItem struct {
	SubjectName   string  `json:"subject_name"`
	ModuleName    string  `json:"module_name"`
	ExerciseName  string  `json:"exercise_name"`
	CurrentStatus string  `json:"current_status"`
	CurrentScore  float64 `json:"current_score"`
	MaxScore      float64 `json:"max_score"`
	AssignmentURL string  `json:"assignment_url"`
}

type SubjectRec struct {
	SubjectName     string    `json:"subject_name"`
	CurrentGrade    float64   `json:"current_grade"`
	TargetScore     float64   `json:"target_score"`
	TargetAchieved  bool      `json:"target_achieved"`
	ProjectedGrade  float64   `json:"projected_grade"`
	RecommendedQueue []RecItem `json:"recommended_queue"`
}

type StudentRecResponse struct {
	Username    string       `json:"username"`
	TargetScore float64      `json:"target_score"`
	Subjects    []SubjectRec `json:"subjects"`
}

type StatusSummary map[string]int

type ExerciseStatusSummary struct {
	ExerciseName string        `json:"exercise_name"`
	Summary      StatusSummary `json:"summary"`
}

type ModuleStatusSummary struct {
	ModuleName string                  `json:"module_name"`
	Summary    StatusSummary           `json:"summary"`
	Exercises  []ExerciseStatusSummary `json:"exercises"`
}

type SubjectStatusSummary struct {
	SubjectName string                `json:"subject_name"`
	Summary     StatusSummary         `json:"summary"`
	Modules     []ModuleStatusSummary `json:"modules"`
}

type StatusCalcRequest struct {
	Students []string `json:"students"`
}

type CourseStatusResponse struct {
	Summary  StatusSummary          `json:"summary"`
	Subjects []SubjectStatusSummary `json:"subjects"`
}
