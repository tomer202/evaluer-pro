package tests

import (
	"fmt"
	"testing"
	"time"

	"evaluer/internal/cache"
	"evaluer/internal/config"
	"evaluer/internal/model"
	"evaluer/internal/service"
)

func setupTestCalculatorWithStudents() (*service.GradeCalculator, *cache.CacheManager) {
	cacheMgr := cache.NewCacheManager(nil)
	configMgr := config.NewConfigManager(nil)

	testConfig := model.CourseConfig{
		Subjects: map[string]model.SubjectConfig{
			"Mathematics": {
				Name:   "Mathematics",
				Weight: 1.0,
				Modules: map[string]model.ModuleConfig{
					"AlgebraBasics": {
						Name:   "AlgebraBasics",
						Weight: 1.0,
						Exercises: map[string]model.ExerciseConfig{
							"QuadraticEq": {Name: "QuadraticEq", Score: 100.0},
						},
					},
				},
			},
		},
	}
	configMgr.SetActiveConfig(testConfig)

	cachedData := &cache.CachedData{
		Subjects: []model.SubjectEntity{
			{ID: 1, Name: "Mathematics", ParentProgramID: 10},
		},
		Modules: []model.ModuleEntity{
			{ID: 10, Name: "AlgebraBasics", ParentSubjectID: 1},
		},
		Exercises: []model.ExerciseEntity{
			{ID: 100, Name: "QuadraticEq", ParentModuleID: 10},
		},
		Students: []model.UserEntity{
			{ID: 1, Username: "alice", FirstName: "Alice", LastName: "Smith", Clearance: 1},
			{ID: 2, Username: "bob", FirstName: "Bob", LastName: "Jones", Clearance: 1},
			{ID: 3, Username: "charlie", FirstName: "Charlie", LastName: "Brown", Clearance: 1},
		},
		Assignments: []model.AssignmentEntity{
			{ID: 1, UserID: 1, ExerciseID: 100, AssignmentStatus: 5, Date: time.Now()}, // Alice: Done
			{ID: 2, UserID: 2, ExerciseID: 100, AssignmentStatus: 2, Date: time.Now()}, // Bob: Redo
		},
		Responses: []model.ResponseEntity{},
		FetchedAt: time.Now(),
	}

	cacheMgr.SetCachedData(cachedData)
	calcEngine := service.NewGradeCalculator(cacheMgr, configMgr)

	return calcEngine, cacheMgr
}

func TestStudentFiltering_CalculateStudent(t *testing.T) {
	calc, _ := setupTestCalculatorWithStudents()

	// Test 1: Fetch Alice
	resAlice, err := calc.CalculateStudent(model.StudentDetailRequest{Username: "alice"})
	if err != nil {
		t.Fatalf("Unexpected error fetching Alice: %v", err)
	}
	if resAlice.StudentID != 1 || resAlice.Username != "alice" {
		t.Errorf("Expected student Alice (ID: 1), got ID: %d, Username: %s", resAlice.StudentID, resAlice.Username)
	}

	// Test 2: Fetch Bob (Must return Bob, NOT Alice)
	resBob, err := calc.CalculateStudent(model.StudentDetailRequest{Username: "bob"})
	if err != nil {
		t.Fatalf("Unexpected error fetching Bob: %v", err)
	}
	if resBob.StudentID != 2 || resBob.Username != "bob" {
		t.Errorf("Expected student Bob (ID: 2), got ID: %d, Username: %s", resBob.StudentID, resBob.Username)
	}

	// Test 3: Case-insensitive Username ("BOB")
	resBobCase, err := calc.CalculateStudent(model.StudentDetailRequest{Username: "BOB"})
	if err != nil {
		t.Fatalf("Unexpected error fetching BOB: %v", err)
	}
	if resBobCase.StudentID != 2 || resBobCase.Username != "bob" {
		t.Errorf("Expected student Bob (ID: 2), got ID: %d, Username: %s", resBobCase.StudentID, resBobCase.Username)
	}

	// Test 4: Nonexistent Student
	_, errNonexistent := calc.CalculateStudent(model.StudentDetailRequest{Username: "nonexistent"})
	if errNonexistent == nil {
		t.Errorf("Expected error for nonexistent student, got nil")
	}
}

func TestStudentFiltering_CalculateRecommendations(t *testing.T) {
	calc, _ := setupTestCalculatorWithStudents()

	// Recommendations for Alice
	recAlice, err := calc.CalculateRecommendations(model.StudentRecRequest{Username: "alice", TargetScore: 70.0})
	if err != nil {
		t.Fatalf("Unexpected error fetching Alice recommendations: %v", err)
	}
	if recAlice.Username != "alice" {
		t.Errorf("Expected username alice in recommendations, got %s", recAlice.Username)
	}

	// Recommendations for Bob (Must return Bob)
	recBob, err := calc.CalculateRecommendations(model.StudentRecRequest{Username: "bob", TargetScore: 70.0})
	if err != nil {
		t.Fatalf("Unexpected error fetching Bob recommendations: %v", err)
	}
	if recBob.Username != "bob" {
		t.Errorf("Expected username bob in recommendations, got %s", recBob.Username)
	}
}

func TestStudentFiltering_ExerciseAndCourseCalc(t *testing.T) {
	calc, _ := setupTestCalculatorWithStudents()

	// Calculate exercise for Bob only
	exRes, err := calc.CalculateExercise(model.ExerciseCalcRequest{
		SubjectName:  "Mathematics",
		ModuleName:   "AlgebraBasics",
		ExerciseName: "QuadraticEq",
		Students:     []string{"bob"},
	})
	if err != nil {
		t.Fatalf("Unexpected error in CalculateExercise: %v", err)
	}
	if len(exRes.Grades) != 1 {
		t.Fatalf("Expected 1 grade result for Bob, got %d", len(exRes.Grades))
	}
	if exRes.Grades[0].Username != "bob" {
		t.Errorf("Expected grade for bob, got %s", exRes.Grades[0].Username)
	}

	// Calculate course for Alice and Bob
	courseRes, err := calc.CalculateCourse(model.CourseCalcRequest{
		Students: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("Unexpected error in CalculateCourse: %v", err)
	}
	if len(courseRes.Students) != 2 {
		t.Fatalf("Expected 2 student course results, got %d", len(courseRes.Students))
	}

	// Calculate course for all students (empty filter)
	allCourseRes, err := calc.CalculateCourse(model.CourseCalcRequest{
		Students: []string{},
	})
	if err != nil {
		t.Fatalf("Unexpected error in CalculateCourse for all students: %v", err)
	}
	if len(allCourseRes.Students) != 3 {
		t.Fatalf("Expected 3 student course results for all, got %d", len(allCourseRes.Students))
	}
}

func TestCalculateStatusBreakdown(t *testing.T) {
	calc, _ := setupTestCalculatorWithStudents()

	statusRes, err := calc.CalculateStatusBreakdown(model.StatusCalcRequest{
		Students: []string{},
	})
	if err != nil {
		t.Fatalf("Unexpected error in CalculateStatusBreakdown: %v", err)
	}

	// Alice has Done (1), Bob has Redo (1), Charlie has no assignment (0).
	// Unopened exercises should NOT be counted as New.
	if statusRes.Summary[model.StatusDone] != 1 {
		t.Errorf("Expected 1 Done assignment, got %d", statusRes.Summary[model.StatusDone])
	}
	if statusRes.Summary[model.StatusRedo] != 1 {
		t.Errorf("Expected 1 Redo assignment, got %d", statusRes.Summary[model.StatusRedo])
	}
	if statusRes.Summary[model.StatusNew] != 0 {
		t.Errorf("Expected 0 New assignments (unopened exercises excluded), got %d", statusRes.Summary[model.StatusNew])
	}
}

func TestLargeScalePerformance(t *testing.T) {
	cacheMgr := cache.NewCacheManager(nil)
	configMgr := config.NewConfigManager(nil)

	// Create 12 subjects, 60 modules, ~4000 exercises, 53 students
	subjectsMap := make(map[string]model.SubjectConfig)
	subjectEntities := make([]model.SubjectEntity, 0, 12)
	moduleEntities := make([]model.ModuleEntity, 0, 60)
	exerciseEntities := make([]model.ExerciseEntity, 0, 4000)

	exID := 1
	modID := 1
	for s := 1; s <= 12; s++ {
		sName := fmt.Sprintf("Subject_%d", s)
		subjectEntities = append(subjectEntities, model.SubjectEntity{ID: s, Name: sName})

		modConfigs := make(map[string]model.ModuleConfig)
		for m := 1; m <= 5; m++ {
			mName := fmt.Sprintf("Mod_%d_%d", s, m)
			moduleEntities = append(moduleEntities, model.ModuleEntity{ID: modID, Name: mName, ParentSubjectID: s})

			exConfigs := make(map[string]model.ExerciseConfig)
			for e := 1; e <= 66; e++ { // ~4000 exercises total
				eName := fmt.Sprintf("Ex_%d", exID)
				exerciseEntities = append(exerciseEntities, model.ExerciseEntity{ID: exID, Name: eName, ParentModuleID: modID})
				exConfigs[eName] = model.ExerciseConfig{Name: eName, Score: 100.0}
				exID++
			}
			modConfigs[mName] = model.ModuleConfig{Name: mName, Weight: 1.0, Exercises: exConfigs}
			modID++
		}
		subjectsMap[sName] = model.SubjectConfig{Name: sName, Weight: 1.0, Modules: modConfigs}
	}

	configMgr.SetActiveConfig(model.CourseConfig{Subjects: subjectsMap})

	// 53 Students
	students := make([]model.UserEntity, 0, 53)
	assignments := make([]model.AssignmentEntity, 0, 53*100)

	assID := 1
	for u := 1; u <= 53; u++ {
		uName := fmt.Sprintf("user_%d", u)
		students = append(students, model.UserEntity{ID: u, Username: uName, Clearance: 1})

		// Each student opens ~100 exercises
		for ex := 1; ex <= 100; ex++ {
			assignments = append(assignments, model.AssignmentEntity{
				ID: assID, UserID: u, ExerciseID: ex, AssignmentStatus: 5, Date: time.Now(),
			})
			assID++
		}
	}

	cacheMgr.SetCachedData(&cache.CachedData{
		Subjects:    subjectEntities,
		Modules:     moduleEntities,
		Exercises:   exerciseEntities,
		Students:    students,
		Assignments: assignments,
		Responses:   nil,
		FetchedAt:   time.Now(),
	})

	calcEngine := service.NewGradeCalculator(cacheMgr, configMgr)

	// Measure CalculateStudent execution time
	start := time.Now()
	_, err := calcEngine.CalculateStudent(model.StudentDetailRequest{Username: "user_1"})
	dur := time.Since(start)
	if err != nil {
		t.Fatalf("CalculateStudent failed: %v", err)
	}
	t.Logf("CalculateStudent (4,000 exercises scale) took: %v", dur)
	if dur > 100*time.Millisecond {
		t.Errorf("CalculateStudent took longer than 100ms: %v", dur)
	}

	// Measure CalculateCourse execution time
	start = time.Now()
	_, err = calcEngine.CalculateCourse(model.CourseCalcRequest{Students: []string{}})
	dur = time.Since(start)
	if err != nil {
		t.Fatalf("CalculateCourse failed: %v", err)
	}
	t.Logf("CalculateCourse (4,000 exercises x 53 students scale) took: %v", dur)
	if dur > 200*time.Millisecond {
		t.Errorf("CalculateCourse took longer than 200ms: %v", dur)
	}
}
