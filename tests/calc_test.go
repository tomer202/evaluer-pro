package tests

import (
	"math"
	"testing"
	"time"

	"evaluer/internal/model"
	"evaluer/internal/service"
)

func TestParseGradeFromContent(t *testing.T) {
	cases := []struct {
		content  string
		expected float64
	}{
		{"80", 80.0},
		{"לא טוב 10", 10.0},
		{"Great job! 95.5", 95.5},
		{"Review comment\n90", 90.0},
		{"No numbers here", 0.0},
	}

	for _, c := range cases {
		val := service.ParseGradeFromContent(c.content)
		if math.Abs(val-c.expected) > 0.001 {
			t.Errorf("For content %q expected %f, got %f", c.content, c.expected, val)
		}
	}
}

func TestMapStatusToString(t *testing.T) {
	if model.MapStatusToString(0) != model.StatusNew {
		t.Errorf("Expected New, got %s", model.MapStatusToString(0))
	}
	if model.MapStatusToString(2) != model.StatusRedo {
		t.Errorf("Expected Redo, got %s", model.MapStatusToString(2))
	}
	if model.MapStatusToString(5) != model.StatusDone {
		t.Errorf("Expected Done, got %s", model.MapStatusToString(5))
	}
}

func TestConfigDataStructures(t *testing.T) {
	cfg := model.CourseConfig{
		Subjects: map[string]model.SubjectConfig{
			"Math": {
				Name:   "Math",
				Weight: 1.0,
				Modules: map[string]model.ModuleConfig{
					"Algebra": {
						Name:   "Algebra",
						Weight: 1.0,
						Exercises: map[string]model.ExerciseConfig{
							"Equations": {Name: "Equations", Score: 100.0},
						},
					},
				},
			},
		},
	}

	if cfg.Subjects["Math"].Weight != 1.0 {
		t.Errorf("Expected Math weight 1.0")
	}
	if cfg.Subjects["Math"].Modules["Algebra"].Exercises["Equations"].Score != 100.0 {
		t.Errorf("Expected Equations score 100.0")
	}
}

func TestTimestampChecking(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)

	if !now.After(earlier) {
		t.Errorf("Time comparison failure")
	}
}
