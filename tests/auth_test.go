package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"evaluer/internal/api"
	"evaluer/internal/cache"
	"evaluer/internal/config"
	"evaluer/internal/model"
	"evaluer/internal/service"
)

func setupTestRouter() (*api.APIHandler, http.Handler) {
	cacheMgr := cache.NewCacheManager(nil)
	cacheMgr.SetCachedData(&cache.CachedData{
		Subjects: []model.SubjectEntity{
			{ID: 1, Name: "Math", Symbol: "MATH"},
		},
		Modules: []model.ModuleEntity{
			{ID: 10, Name: "Algebra", ParentSubjectID: 1},
		},
		Exercises: []model.ExerciseEntity{
			{ID: 100, Name: "Ex1", ParentModuleID: 10},
		},
		Students: []model.UserEntity{
			{ID: 1, Username: "stu1", Clearance: 1},
			{ID: 2, Username: "stu2", Clearance: 1},
			{ID: 3, Username: "stu3", Clearance: 1},
			{ID: 4, Username: "stu4", Clearance: 1},
			{ID: 5, Username: "stu5", Clearance: 1},
			{ID: 6, Username: "stu6", Clearance: 1},
		},
	})

	configMgr := config.NewConfigManager(nil)
	configMgr.SetActiveConfig(model.CourseConfig{
		Subjects: map[string]model.SubjectConfig{
			"Math": {
				Name:   "Math",
				Weight: 1.0,
				Modules: map[string]model.ModuleConfig{
					"Algebra": {
						Name:   "Algebra",
						Weight: 1.0,
						Exercises: map[string]model.ExerciseConfig{
							"Ex1": {Name: "Ex1", Score: 100.0},
						},
					},
				},
			},
		},
	})

	calc := service.NewGradeCalculator(cacheMgr, configMgr)
	r := api.SetupRouter(calc, configMgr)
	return nil, r
}

func getAuthToken(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Login failed for user %s: status %d, body: %s", username, w.Code, w.Body.String())
	}

	var resp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Token == "" {
		t.Fatalf("Empty token returned for user %s", username)
	}
	return resp.Token
}

func TestLoginEndpoint(t *testing.T) {
	_, r := setupTestRouter()

	// 1. Admin valid login
	adminToken := getAuthToken(t, r, "admin", "QWEqwe123")
	if adminToken == "" {
		t.Errorf("Expected valid token for admin")
	}

	// 2. Chief valid login
	chiefToken := getAuthToken(t, r, "chief", "EWQewq321")
	if chiefToken == "" {
		t.Errorf("Expected valid token for chief")
	}

	// 3. Invalid credentials
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrongpassword"})
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for bad password, got %d", w.Code)
	}
}

func TestCalcExercise_AuthPermissions(t *testing.T) {
	_, r := setupTestRouter()
	adminToken := getAuthToken(t, r, "admin", "QWEqwe123")
	chiefToken := getAuthToken(t, r, "chief", "EWQewq321")

	// Payload generator
	makeReq := func(students []string) *http.Request {
		b, _ := json.Marshal(map[string]interface{}{
			"subject_name":  "Math",
			"module_name":   "Algebra",
			"exercise_name": "Ex1",
			"students":      students,
		})
		req, _ := http.NewRequest("POST", "/api/v1/calc/exercise", bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	// 1. Unauthenticated request -> 401 Unauthorized
	req1 := makeReq([]string{"stu1"})
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated /calc/exercise, got %d", w1.Code)
	}

	// 2. Admin with 0 students -> 403 Forbidden
	req2 := makeReq([]string{})
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for admin with 0 students, got %d", w2.Code)
	}

	// 3. Admin with 6 students (>5) -> 403 Forbidden
	req3 := makeReq([]string{"stu1", "stu2", "stu3", "stu4", "stu5", "stu6"})
	req3.Header.Set("Authorization", "Bearer "+adminToken)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for admin with 6 students, got %d", w3.Code)
	}

	// 4. Admin with 3 students (1..5) -> 200 OK
	req4 := makeReq([]string{"stu1", "stu2", "stu3"})
	req4.Header.Set("Authorization", "Bearer "+adminToken)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Errorf("Expected 200 for admin with 3 students, got %d", w4.Code)
	}

	// 5. Chief with any students -> 200 OK
	req5 := makeReq([]string{"stu1", "stu2", "stu3", "stu4", "stu5", "stu6"})
	req5.Header.Set("Authorization", "Bearer "+chiefToken)
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Errorf("Expected 200 for chief with 6 students, got %d", w5.Code)
	}
}

func TestCalcCourse_AuthPermissions(t *testing.T) {
	_, r := setupTestRouter()
	adminToken := getAuthToken(t, r, "admin", "QWEqwe123")
	chiefToken := getAuthToken(t, r, "chief", "EWQewq321")

	makeReq := func() *http.Request {
		b, _ := json.Marshal(map[string]interface{}{"students": []string{}})
		req, _ := http.NewRequest("POST", "/api/v1/calc/course", bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	// 1. Unauthenticated -> 401 Unauthorized
	req1 := makeReq()
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated /calc/course, got %d", w1.Code)
	}

	// 2. Admin user -> 403 Forbidden
	req2 := makeReq()
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for admin on /calc/course, got %d", w2.Code)
	}

	// 3. Chief user -> 200 OK
	req3 := makeReq()
	req3.Header.Set("Authorization", "Bearer "+chiefToken)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("Expected 200 for chief on /calc/course, got %d", w3.Code)
	}
}

func TestGetStudentsRoute(t *testing.T) {
	_, r := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/students", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for unauthenticated GET /api/v1/students, got %d", w.Code)
	}

	var students []model.StudentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &students); err != nil {
		t.Fatalf("Failed to unmarshal /api/v1/students response: %v", err)
	}

	if len(students) != 6 {
		t.Errorf("Expected 6 students in response, got %d", len(students))
	}

	if students[0].StudentID != 1 || students[0].Username != "stu1" {
		t.Errorf("Unexpected first student fields: %+v", students[0])
	}
}

func TestSwaggerOfflineAssets(t *testing.T) {
	_, r := setupTestRouter()

	assets := []string{
		"/docs",
		"/swagger-assets/swagger-ui.css",
		"/swagger-assets/swagger-ui-bundle.js",
		"/swagger-assets/swagger-ui-standalone-preset.js",
		"/openapi.json",
	}

	for _, path := range assets {
		req, _ := http.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for Swagger asset %s, got %d", path, w.Code)
		}
		if len(w.Body.Bytes()) == 0 {
			t.Errorf("Expected non-empty body for Swagger asset %s", path)
		}
	}
}
