package tests

import (
	"math"
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
