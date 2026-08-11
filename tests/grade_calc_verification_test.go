package tests

import (
	"math"
	"strings"
	"testing"
	"time"

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
