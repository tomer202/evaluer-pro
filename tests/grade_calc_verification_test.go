package tests

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"evaluer/internal/api"
	"evaluer/internal/cache"
	"evaluer/internal/config"
	"evaluer/internal/model"
	"evaluer/internal/service"
)

func TestParseGradeFromContent_FractionCases(t *testing.T) {
	cases := []struct {
		content  string
		expected float64
	}{
		{"80", 80.0},
		{"לא טוב 10", 10.0},
		{"Great job! 95.5", 95.5},
		{"Review comment\n90", 90.0},
		{"No numbers here", 0.0},
		{"Grade: 85/100", 85.0},
		{"ציון: 92/100", 92.0},
		{"Score: 75 out of 100", 75.0},
		{"88 / 100", 88.0},
		{"Final grade: 65.5/100", 65.5},
	}

	for _, c := range cases {
		val := service.ParseGradeFromContent(c.content)
		if math.Abs(val-c.expected) > 0.001 {
			t.Errorf("For content %q expected %f, got %f", c.content, c.expected, val)
		}
	}
}

func TestGradeCalculation_UnopenedModuleHandling(t *testing.T) {
	cacheMgr := cache.NewCacheManager(nil)
	configMgr := config.NewConfigManager(nil)

	// Subject "Math" with 2 modules (Module1 weight 1.0, Module2 weight 1.0)
	configMgr.SetActiveConfig(model.CourseConfig{
		Subjects: map[string]model.SubjectConfig{
			"Math": {
				Name:   "Math",
				Weight: 1.0,
				Modules: map[string]model.ModuleConfig{
					"Mod1": {
						Name:   "Mod1",
						Weight: 1.0,
						Exercises: map[string]model.ExerciseConfig{
							"Ex1": {Name: "Ex1", Score: 100.0},
						},
					},
					"Mod2": {
						Name:   "Mod2",
						Weight: 1.0,
						Exercises: map[string]model.ExerciseConfig{
							"Ex2": {Name: "Ex2", Score: 100.0},
						},
					},
				},
			},
		},
	})

	// Student Alice has ONLY opened Ex1 in Mod1 and scored 100% (Done).
	// Alice has NEVER opened Mod2 (Ex2).
	cacheMgr.SetCachedData(&cache.CachedData{
		Subjects:  []model.SubjectEntity{{ID: 1, Name: "Math"}},
		Modules:   []model.ModuleEntity{{ID: 1, Name: "Mod1", ParentSubjectID: 1}, {ID: 2, Name: "Mod2", ParentSubjectID: 1}},
		Exercises: []model.ExerciseEntity{{ID: 1, Name: "Ex1", ParentModuleID: 1}, {ID: 2, Name: "Ex2", ParentModuleID: 2}},
		Students:  []model.UserEntity{{ID: 10, Username: "alice", Clearance: 1}},
		Assignments: []model.AssignmentEntity{
			{ID: 100, UserID: 10, ExerciseID: 1, AssignmentStatus: model.AssignmentStatusDone, Date: time.Now()},
		},
		Responses: nil,
		FetchedAt: time.Now(),
	})

	calc := service.NewGradeCalculator(cacheMgr, configMgr)

	detail, err := calc.CalculateStudent(model.StudentDetailRequest{Username: "alice"})
	if err != nil {
		t.Fatalf("CalculateStudent failed: %v", err)
	}

	t.Logf("Alice TotalGrade: %f, Subjects count: %d", detail.TotalGrade, len(detail.Subjects))
	if len(detail.Subjects) > 0 {
		t.Logf("Math SubjectGrade: %f, Modules count: %d", detail.Subjects[0].SubjectGrade, len(detail.Subjects[0].Modules))
	}
}

func TestScoreCalculation_UserFormula(t *testing.T) {
	cacheMgr := cache.NewCacheManager(nil)
	configMgr := config.NewConfigManager(nil)

	// Math -> Mod1 (weight 0.6): Ex1 (score 25), Ex2 (score 25), Ex3 (score 50)
	//        Mod2 (weight 0.4): Ex4 (score 100)
	configMgr.SetActiveConfig(model.CourseConfig{
		Subjects: map[string]model.SubjectConfig{
			"Math": {
				Name:   "Math",
				Weight: 1.0,
				Modules: map[string]model.ModuleConfig{
					"Mod1": {
						Name:   "Mod1",
						Weight: 0.6,
						Exercises: map[string]model.ExerciseConfig{
							"Ex1": {Name: "Ex1", Score: 25.0},
							"Ex2": {Name: "Ex2", Score: 25.0},
							"Ex3": {Name: "Ex3", Score: 50.0},
						},
					},
					"Mod2": {
						Name:   "Mod2",
						Weight: 0.4,
						Exercises: map[string]model.ExerciseConfig{
							"Ex4": {Name: "Ex4", Score: 100.0},
						},
					},
				},
			},
		},
	})

	// Student Bob:
	// Ex1: Done (100%) -> 25.0
	// Ex2: Redo (80%) -> 20.0
	// Ex3: Unopened -> 0.0
	// Mod1 grade = 25 + 20 + 0 = 45.0
	// Ex4: Done (100%) -> 100.0
	// Mod2 grade = 100.0
	// Math grade = (45.0 * 0.6 + 100.0 * 0.4) / (0.6 + 0.4) = 67.0
	cacheMgr.SetCachedData(&cache.CachedData{
		Subjects:  []model.SubjectEntity{{ID: 1, Name: "Math"}},
		Modules:   []model.ModuleEntity{{ID: 1, Name: "Mod1", ParentSubjectID: 1}, {ID: 2, Name: "Mod2", ParentSubjectID: 1}},
		Exercises: []model.ExerciseEntity{{ID: 1, Name: "Ex1", ParentModuleID: 1}, {ID: 2, Name: "Ex2", ParentModuleID: 1}, {ID: 3, Name: "Ex3", ParentModuleID: 1}, {ID: 4, Name: "Ex4", ParentModuleID: 2}},
		Students:  []model.UserEntity{{ID: 10, Username: "bob", Clearance: 1}},
		Assignments: []model.AssignmentEntity{
			{ID: 101, UserID: 10, ExerciseID: 1, AssignmentStatus: model.AssignmentStatusDone, Date: time.Now()},
			{ID: 102, UserID: 10, ExerciseID: 2, AssignmentStatus: model.AssignmentStatusRedo, Date: time.Now()},
			{ID: 104, UserID: 10, ExerciseID: 4, AssignmentStatus: model.AssignmentStatusDone, Date: time.Now()},
		},
		Responses: []model.ResponseEntity{
			{ID: 1, ThreadID: 102, UserID: 2, Date: time.Now(), SegelOnly: true, ResponseType: "Redo", Content: "80"},
		},
		FetchedAt: time.Now(),
	})

	calc := service.NewGradeCalculator(cacheMgr, configMgr)

	detail, err := calc.CalculateStudent(model.StudentDetailRequest{Username: "bob"})
	if err != nil {
		t.Fatalf("CalculateStudent failed: %v", err)
	}

	if math.Abs(detail.TotalGrade-67.0) > 0.01 {
		t.Errorf("Expected TotalGrade 67.0, got %f", detail.TotalGrade)
	}

	if len(detail.Subjects) > 0 {
		sub := detail.Subjects[0]
		if math.Abs(sub.SubjectGrade-67.0) > 0.01 {
			t.Errorf("Expected SubjectGrade 67.0, got %f", sub.SubjectGrade)
		}
		for _, m := range sub.Modules {
			if m.ModuleName == "Mod1" && math.Abs(m.ModuleGrade-45.0) > 0.01 {
				t.Errorf("Expected Mod1 grade 45.0, got %f", m.ModuleGrade)
			}
			if m.ModuleName == "Mod2" && math.Abs(m.ModuleGrade-100.0) > 0.01 {
				t.Errorf("Expected Mod2 grade 100.0, got %f", m.ModuleGrade)
			}
		}
	}
}

func TestJSONToYAML_SequencePreservation(t *testing.T) {
	jsonInput := []byte(`{
  "weight": 1.0,
  "name": "Math",
  "modules": {
    "Algebra": {
      "weight": 0.5,
      "name": "Algebra"
    }
  }
}`)

	yamlOutput, err := config.ConvertJSONToYAML(jsonInput)
	if err != nil {
		t.Fatalf("ConvertJSONToYAML failed: %v", err)
	}

	yamlStr := string(yamlOutput)
	t.Logf("Converted YAML:\n%s", yamlStr)

	// Check that "weight" appears before "name" in the YAML string
	weightIdx := strings.Index(yamlStr, "weight")
	nameIdx := strings.Index(yamlStr, "name")

	if weightIdx == -1 || nameIdx == -1 {
		t.Fatalf("Missing weight or name in YAML output: %s", yamlStr)
	}

	if weightIdx > nameIdx {
		t.Errorf("Expected key 'weight' before 'name', but 'name' appeared first! Output:\n%s", yamlStr)
	}
}

func TestHealthCheckRoute(t *testing.T) {
	cacheMgr := cache.NewCacheManager(nil)
	cacheMgr.SetCachedData(&cache.CachedData{})
	configMgr := config.NewConfigManager(nil)
	calc := service.NewGradeCalculator(cacheMgr, configMgr)

	r := api.SetupRouter(calc, configMgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api-health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api-health, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"hive":"alive"`) && !strings.Contains(body, `"status":"healthy"`) {
		t.Errorf("Unexpected body for /api-health: %s", body)
	}
}

func TestDoneStatusIsCheckedAlwaysTrue(t *testing.T) {
	cacheMgr := cache.NewCacheManager(nil)
	configMgr := config.NewConfigManager(nil)

	now := time.Now()
	cacheMgr.SetCachedData(&cache.CachedData{
		Subjects:  []model.SubjectEntity{{ID: 1, Name: "Math"}},
		Modules:   []model.ModuleEntity{{ID: 1, Name: "Mod1", ParentSubjectID: 1}},
		Exercises: []model.ExerciseEntity{{ID: 1, Name: "Ex1", ParentModuleID: 1}},
		Students:  []model.UserEntity{{ID: 10, Username: "charlie", Clearance: 1}},
		Assignments: []model.AssignmentEntity{
			{ID: 100, UserID: 10, ExerciseID: 1, AssignmentStatus: model.AssignmentStatusDone, Date: now},
		},
		Responses: []model.ResponseEntity{
			{ID: 1, ThreadID: 100, UserID: 10, Date: now.Add(1 * time.Hour), SegelOnly: false, ResponseType: "Submission", Content: ""},
			{ID: 2, ThreadID: 100, UserID: 2, Date: now, SegelOnly: true, ResponseType: "Redo", Content: "100"},
		},
		FetchedAt: now,
	})

	calc := service.NewGradeCalculator(cacheMgr, configMgr)

	resp, err := calc.CalculateExercise(model.ExerciseCalcRequest{
		SubjectName:  "Math",
		ModuleName:   "Mod1",
		ExerciseName: "Ex1",
		Students:     []string{"charlie"},
	})
	if err != nil {
		t.Fatalf("CalculateExercise failed: %v", err)
	}

	if len(resp.Grades) != 1 {
		t.Fatalf("Expected 1 grade result, got %d", len(resp.Grades))
	}

	g := resp.Grades[0]
	if g.Status != model.StatusDone {
		t.Errorf("Expected Status Done, got %s", g.Status)
	}
	if !g.IsChecked {
		t.Errorf("Expected IsChecked to be TRUE for Done assignment status, got false!")
	}
}

func TestCalcResponse_ConfigSequenceOrdering(t *testing.T) {
	cacheMgr := cache.NewCacheManager(nil)
	configMgr := config.NewConfigManager(nil)

	// Set config raw YAML with custom order: Physics before Math, Thermodynamics before Mechanics, ExB before ExA
	yamlContent := `subjects:
  Physics:
    weight: 1.0
    modules:
      Thermodynamics:
        weight: 1.0
        exercises:
          ExB:
            name: "ExB"
            score: 100
          ExA:
            name: "ExA"
            score: 100
  Math:
    weight: 1.0
    modules:
      Algebra:
        weight: 1.0
        exercises:
          Ex1:
            name: "Ex1"
            score: 100
`
	if err := configMgr.SaveRawJSON("", []byte(yamlContent)); err != nil {
		// Save directly to active config rawYAML
		configMgr.SetActiveConfig(model.CourseConfig{})
	}

	now := time.Now()
	cacheMgr.SetCachedData(&cache.CachedData{
		Subjects: []model.SubjectEntity{
			{ID: 1, Name: "Math"},
			{ID: 2, Name: "Physics"},
		},
		Modules: []model.ModuleEntity{
			{ID: 10, Name: "Algebra", ParentSubjectID: 1},
			{ID: 20, Name: "Thermodynamics", ParentSubjectID: 2},
		},
		Exercises: []model.ExerciseEntity{
			{ID: 100, Name: "Ex1", ParentModuleID: 10},
			{ID: 200, Name: "ExA", ParentModuleID: 20},
			{ID: 201, Name: "ExB", ParentModuleID: 20},
		},
		Students: []model.UserEntity{
			{ID: 5, Username: "dave", Clearance: 1},
		},
		Assignments: []model.AssignmentEntity{
			{ID: 1000, UserID: 5, ExerciseID: 100, AssignmentStatus: model.AssignmentStatusDone, Date: now},
			{ID: 2000, UserID: 5, ExerciseID: 200, AssignmentStatus: model.AssignmentStatusDone, Date: now},
			{ID: 2001, UserID: 5, ExerciseID: 201, AssignmentStatus: model.AssignmentStatusDone, Date: now},
		},
		Responses: []model.ResponseEntity{},
		FetchedAt: now,
	})

	calc := service.NewGradeCalculator(cacheMgr, configMgr)

	studentResp, err := calc.CalculateStudent(model.StudentDetailRequest{Username: "dave"})
	if err != nil {
		t.Fatalf("CalculateStudent failed: %v", err)
	}

	if len(studentResp.Subjects) != 2 {
		t.Fatalf("Expected 2 subjects, got %d", len(studentResp.Subjects))
	}

	// Verify Physics is returned BEFORE Math matching config.yaml sequence!
	if studentResp.Subjects[0].SubjectName != "Physics" || studentResp.Subjects[1].SubjectName != "Math" {
		t.Errorf("Expected Physics before Math in CalculateStudent response! Got: %s, %s",
			studentResp.Subjects[0].SubjectName, studentResp.Subjects[1].SubjectName)
	}

	// Verify ExB is returned BEFORE ExA under Thermodynamics matching config.yaml sequence!
	physModules := studentResp.Subjects[0].Modules
	if len(physModules) > 0 && len(physModules[0].Exercises) == 2 {
		exes := physModules[0].Exercises
		if exes[0].ExerciseName != "ExB" || exes[1].ExerciseName != "ExA" {
			t.Errorf("Expected ExB before ExA under Thermodynamics! Got: %s, %s",
				exes[0].ExerciseName, exes[1].ExerciseName)
		}
	}
}

func TestConfigGenerate_Ordering(t *testing.T) {
	// Verify SubjectEntity, ModuleEntity, ExerciseEntity struct fields
	sub := model.SubjectEntity{ID: 1, Name: "Math", Symbol: "MATH", ParentProgramID: 10}
	if sub.Symbol != "MATH" {
		t.Errorf("Expected SubjectEntity Symbol 'MATH', got '%s'", sub.Symbol)
	}

	mod := model.ModuleEntity{ID: 1, Name: "Algebra", Order: "01", ParentSubjectID: 1}
	if mod.Order != "01" {
		t.Errorf("Expected ModuleEntity Order '01', got '%s'", mod.Order)
	}

	ex := model.ExerciseEntity{ID: 1, Name: "Ex1", Order: "001", ParentModuleID: 1}
	if ex.Order != "001" {
		t.Errorf("Expected ExerciseEntity Order '001', got '%s'", ex.Order)
	}
}

