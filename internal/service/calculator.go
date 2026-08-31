package service

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"evaluer/internal/cache"
	"evaluer/internal/config"
	"evaluer/internal/model"
)

func roundToTwoDecimals(val float64) float64 {
	return math.Round(val*100.0) / 100.0
}

func makeAssignmentKey(userID, exerciseID int) uint64 {
	return (uint64(userID) << 32) | uint64(exerciseID)
}

var (
	fractionRegex = regexp.MustCompile(`(?i)\b([0-9]+(?:\.[0-9]+)?)\s*(?:/|out of|מתוך)\s*([0-9]+(?:\.[0-9]+)?)\b`)
	numberRegex   = regexp.MustCompile(`\b([0-9]+(?:\.[0-9]+)?)\b`)
)

type GradeCalculator struct {
	cacheMgr  *cache.CacheManager
	configMgr *config.ConfigManager
}

func NewGradeCalculator(cacheMgr *cache.CacheManager, configMgr *config.ConfigManager) *GradeCalculator {
	return &GradeCalculator{
		cacheMgr:  cacheMgr,
		configMgr: configMgr,
	}
}

func (c *GradeCalculator) IsHiveAlive() bool {
	if c == nil || c.cacheMgr == nil {
		return false
	}
	return c.cacheMgr.IsHiveAlive()
}

func getHiveBaseURL() string {
	url := os.Getenv("HIVE_BASE_URL")
	if url == "" {
		url = "https://20.13.2.62"
	}
	return strings.TrimRight(url, "/")
}



type PreparedAssignment struct {
	Assignment model.AssignmentEntity
	Responses  []model.ResponseEntity
}

type PreparedData struct {
	Config          model.CourseConfig
	Students        []model.UserEntity
	Subjects        map[int]model.SubjectEntity
	OrderedSubjects []model.SubjectEntity
	Modules         map[int]model.ModuleEntity
	Exercises       map[int]model.ExerciseEntity
	SubjectByName   map[string]model.SubjectEntity
	ModuleByName    map[string]model.ModuleEntity
	ExerciseByName  map[string]model.ExerciseEntity
	SubjectModules  map[int][]model.ModuleEntity
	ModuleExercises map[int][]model.ExerciseEntity
	// Key: UserID -> ExerciseID -> PreparedAssignment
	UserAssignments map[int]map[int]PreparedAssignment
	// Composite Key: (UserID << 32) | ExerciseID -> PreparedAssignment
	AssignmentMap map[uint64]PreparedAssignment
}

func (p *PreparedData) GetAssignment(userID, exerciseID int) (PreparedAssignment, bool) {
	pa, ok := p.AssignmentMap[makeAssignmentKey(userID, exerciseID)]
	return pa, ok
}

func (c *GradeCalculator) prepareData(targetStudents []string) (*PreparedData, error) {
	cd, err := c.cacheMgr.GetOrFetchData(targetStudents)
	if err != nil {
		return nil, err
	}

	cfg := c.configMgr.GetActiveConfig()

	filteredStudents := make([]model.UserEntity, 0)
	if len(targetStudents) > 0 {
		targetMap := make(map[string]bool)
		for _, u := range targetStudents {
			trimmed := strings.TrimSpace(u)
			if trimmed != "" {
				targetMap[strings.ToLower(trimmed)] = true
			}
		}
		if len(targetMap) > 0 {
			for _, s := range cd.Students {
				if targetMap[strings.ToLower(s.Username)] {
					filteredStudents = append(filteredStudents, s)
				}
			}
		} else {
			filteredStudents = cd.Students
		}
	} else {
		filteredStudents = cd.Students
	}

	prep := &PreparedData{
		Config:          cfg,
		Students:        filteredStudents,
		Subjects:        make(map[int]model.SubjectEntity, len(cd.Subjects)),
		OrderedSubjects: make([]model.SubjectEntity, 0, len(cd.Subjects)),
		Modules:         make(map[int]model.ModuleEntity, len(cd.Modules)),
		Exercises:       make(map[int]model.ExerciseEntity, len(cd.Exercises)),
		SubjectByName:   make(map[string]model.SubjectEntity, len(cd.Subjects)),
		ModuleByName:    make(map[string]model.ModuleEntity, len(cd.Modules)),
		ExerciseByName:  make(map[string]model.ExerciseEntity, len(cd.Exercises)),
		SubjectModules:  make(map[int][]model.ModuleEntity, len(cd.Subjects)),
		ModuleExercises: make(map[int][]model.ExerciseEntity, len(cd.Modules)),
		UserAssignments: make(map[int]map[int]PreparedAssignment, len(filteredStudents)),
		AssignmentMap:   make(map[uint64]PreparedAssignment, len(cd.Assignments)),
	}

	for _, s := range cd.Subjects {
		prep.Subjects[s.ID] = s
		prep.SubjectByName[s.Name] = s
	}

	for _, m := range cd.Modules {
		prep.Modules[m.ID] = m
		prep.ModuleByName[m.Name] = m
		prep.SubjectModules[m.ParentSubjectID] = append(prep.SubjectModules[m.ParentSubjectID], m)
	}

	for _, e := range cd.Exercises {
		prep.Exercises[e.ID] = e
		prep.ExerciseByName[e.Name] = e
		prep.ModuleExercises[e.ParentModuleID] = append(prep.ModuleExercises[e.ParentModuleID], e)
	}

	// Align Subjects, Modules, and Exercises according to config.yaml sequence
	seq := c.configMgr.GetConfigSequence()

	// 1. Order Subjects matching config.yaml sequence
	seenSubjects := make(map[int]bool)
	if len(seq.Subjects) > 0 {
		for _, sName := range seq.Subjects {
			if s, ok := prep.SubjectByName[sName]; ok {
				if !seenSubjects[s.ID] {
					prep.OrderedSubjects = append(prep.OrderedSubjects, s)
					seenSubjects[s.ID] = true
				}
			}
		}
	}
	for _, s := range cd.Subjects {
		if !seenSubjects[s.ID] {
			prep.OrderedSubjects = append(prep.OrderedSubjects, s)
			seenSubjects[s.ID] = true
		}
	}

	// 2. Order Modules for each Subject matching config.yaml sequence
	for _, s := range cd.Subjects {
		mods := prep.SubjectModules[s.ID]
		if len(mods) <= 1 {
			continue
		}
		modListInConfig := seq.Modules[s.Name]
		if len(modListInConfig) == 0 {
			continue
		}

		modByName := make(map[string]model.ModuleEntity, len(mods))
		for _, m := range mods {
			modByName[m.Name] = m
		}

		orderedMods := make([]model.ModuleEntity, 0, len(mods))
		seenMods := make(map[int]bool)

		for _, mName := range modListInConfig {
			if m, ok := modByName[mName]; ok {
				if !seenMods[m.ID] {
					orderedMods = append(orderedMods, m)
					seenMods[m.ID] = true
				}
			}
		}
		for _, m := range mods {
			if !seenMods[m.ID] {
				orderedMods = append(orderedMods, m)
				seenMods[m.ID] = true
			}
		}
		prep.SubjectModules[s.ID] = orderedMods
	}

	// 3. Order Exercises for each Module matching config.yaml sequence
	for _, m := range cd.Modules {
		exes := prep.ModuleExercises[m.ID]
		if len(exes) <= 1 {
			continue
		}
		exListInConfig := seq.Exercises[m.Name]
		if len(exListInConfig) == 0 {
			continue
		}

		exByName := make(map[string]model.ExerciseEntity, len(exes))
		for _, e := range exes {
			exByName[e.Name] = e
		}

		orderedExes := make([]model.ExerciseEntity, 0, len(exes))
		seenExes := make(map[int]bool)

		for _, eName := range exListInConfig {
			if e, ok := exByName[eName]; ok {
				if !seenExes[e.ID] {
					orderedExes = append(orderedExes, e)
					seenExes[e.ID] = true
				}
			}
		}
		for _, e := range exes {
			if !seenExes[e.ID] {
				orderedExes = append(orderedExes, e)
				seenExes[e.ID] = true
			}
		}
		prep.ModuleExercises[m.ID] = orderedExes
	}

	// Map responses by ThreadID
	respMap := make(map[int][]model.ResponseEntity, len(cd.Assignments))
	for _, r := range cd.Responses {
		respMap[r.ThreadID] = append(respMap[r.ThreadID], r)
	}

	// Map assignments by 64-bit composite key and UserID -> ExerciseID
	for _, a := range cd.Assignments {
		pa := PreparedAssignment{
			Assignment: a,
			Responses:  respMap[a.ID],
		}
		key := makeAssignmentKey(a.UserID, a.ExerciseID)
		prep.AssignmentMap[key] = pa

		if prep.UserAssignments[a.UserID] == nil {
			prep.UserAssignments[a.UserID] = make(map[int]PreparedAssignment)
		}
		prep.UserAssignments[a.UserID][a.ExerciseID] = pa
	}

	return prep, nil
}

// ParseGradeFromContent extracts numerical grade from segel review content string
func ParseGradeFromContent(content string) float64 {
	g, _ := ParseGradeFromContentExt(content)
	return g
}

// ParseGradeFromContentExt extracts numerical grade and reports if a valid grade pattern was found
func ParseGradeFromContentExt(content string) (float64, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0.0, false
	}

	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		// 1. Check for fraction patterns (e.g., "85/100", "85 out of 100", "85 מתוך 100")
		if fracMatches := fractionRegex.FindAllStringSubmatch(line, -1); len(fracMatches) > 0 {
			lastFrac := fracMatches[len(fracMatches)-1]
			num, err1 := strconv.ParseFloat(lastFrac[1], 64)
			den, err2 := strconv.ParseFloat(lastFrac[2], 64)
			if err1 == nil && err2 == nil && den > 0 {
				calcGrade := (num / den) * 100.0
				if calcGrade >= 0 && calcGrade <= 100 {
					return roundToTwoDecimals(calcGrade), true
				}
			}
		}

		// 2. Check for single numbers (e.g., "Grade: 85")
		matches := numberRegex.FindAllString(line, -1)
		if len(matches) > 0 {
			for mIdx := len(matches) - 1; mIdx >= 0; mIdx-- {
				if val, err := strconv.ParseFloat(matches[mIdx], 64); err == nil {
					if val >= 0 && val <= 100 {
						return roundToTwoDecimals(val), true
					}
				}
			}
		}
	}
	return 0.0, false
}

func calculateAssignmentGradeInfo(
	pa *PreparedAssignment,
	studentID int,
	subID int,
	modID int,
	exID int,
	maxScore float64,
) (rawGrade float64, score float64, review string, status string, url string, redoCount int, isChecked bool) {
	baseURL := getHiveBaseURL()
	url = fmt.Sprintf("%s/course/%d/%d/%d#%d", baseURL, subID, modID, exID, studentID)

	if pa == nil {
		status = model.StatusNew
		return 0.0, 0.0, "", status, url, 0, true
	}

	assign := pa.Assignment
	status = model.MapStatusToString(assign.AssignmentStatus)

	var latestStudentDate time.Time
	var latestSegelDate time.Time
	hasSegelReview := false
	var segelGrade float64
	hasSegelGrade := false

	for _, r := range pa.Responses {
		if r.SegelOnly {
			hasSegelReview = true
			if r.Date.After(latestSegelDate) {
				latestSegelDate = r.Date
			}
			if strings.EqualFold(r.ResponseType, "Redo") || strings.EqualFold(r.ResponseType, "REDO") {
				redoCount++
			}
			if r.Content != "" {
				review = r.Content
				if g, ok := ParseGradeFromContentExt(r.Content); ok {
					segelGrade = g
					hasSegelGrade = true
				}
			}
		} else {
			if r.Date.After(latestStudentDate) {
				latestStudentDate = r.Date
			}
		}
	}

	if assign.AssignmentStatus == model.AssignmentStatusDone {
		isChecked = true
	} else if hasSegelReview {
		if latestStudentDate.IsZero() {
			isChecked = true
		} else {
			isChecked = !latestStudentDate.After(latestSegelDate)
		}
	} else {
		if assign.AssignmentStatus == model.AssignmentStatusNew {
			isChecked = true
		} else {
			isChecked = false
		}
	}

	switch assign.AssignmentStatus {
	case model.AssignmentStatusDone:
		rawGrade = 100.0
	default:
		if hasSegelGrade {
			rawGrade = segelGrade
		} else {
			rawGrade = 0.0
		}
	}

	rawGrade = roundToTwoDecimals(rawGrade)
	score = roundToTwoDecimals((rawGrade / 100.0) * maxScore)
	return rawGrade, score, review, status, url, redoCount, isChecked
}

// ---------------- API CALCULATION HANDLERS ----------------

func (c *GradeCalculator) CalculateExercise(req model.ExerciseCalcRequest) (*model.ExerciseCalcResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	sub, ok := prep.SubjectByName[req.SubjectName]
	if !ok {
		return nil, fmt.Errorf("subject %s not found", req.SubjectName)
	}
	mod, ok := prep.ModuleByName[req.ModuleName]
	if !ok {
		return nil, fmt.Errorf("module %s not found", req.ModuleName)
	}
	ex, ok := prep.ExerciseByName[req.ExerciseName]
	if !ok {
		return nil, fmt.Errorf("exercise %s not found", req.ExerciseName)
	}

	maxScore := 100.0
	if sc, ok := prep.Config.Subjects[sub.Name]; ok {
		if mc, ok := sc.Modules[mod.Name]; ok {
			if ec, ok := mc.Exercises[ex.Name]; ok {
				maxScore = ec.Score
			}
		}
	}

	resp := &model.ExerciseCalcResponse{
		SubjectName:  sub.Name,
		ModuleName:   mod.Name,
		ExerciseName: ex.Name,
		Grades:       make([]model.ExerciseGradeResponse, 0),
	}

	for _, stu := range prep.Students {
		userAssMap := prep.UserAssignments[stu.ID]
		pa, hasAssignment := userAssMap[ex.ID]

		// Unopened exercise filtering rule: ignore exercises never opened by student!
		if !hasAssignment {
			continue
		}

		rawGrade, score, review, status, url, redoCount, isChecked := calculateAssignmentGradeInfo(
			&pa, stu.ID, sub.ID, mod.ID, ex.ID, maxScore,
		)

		resp.Grades = append(resp.Grades, model.ExerciseGradeResponse{
			StudentID:     stu.ID,
			Username:      stu.Username,
			FirstName:     stu.FirstName,
			LastName:      stu.LastName,
			RawGrade:      rawGrade,
			Score:         score,
			Review:        review,
			Status:        status,
			AssignmentURL: url,
			RedoCount:     redoCount,
			IsChecked:     isChecked,
		})
	}

	return resp, nil
}

func (c *GradeCalculator) CalculateModule(req model.ModuleCalcRequest) (*model.ModuleCalcResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	sub, ok := prep.SubjectByName[req.SubjectName]
	if !ok {
		return nil, fmt.Errorf("subject %s not found", req.SubjectName)
	}
	mod, ok := prep.ModuleByName[req.ModuleName]
	if !ok {
		return nil, fmt.Errorf("module %s not found", req.ModuleName)
	}

	var moduleExercises []model.ExerciseEntity
	for _, ex := range prep.Exercises {
		if ex.ParentModuleID == mod.ID {
			moduleExercises = append(moduleExercises, ex)
		}
	}

	resp := &model.ModuleCalcResponse{
		SubjectName: sub.Name,
		ModuleName:  mod.Name,
		Students:    make([]model.StudentModuleGrade, 0),
	}

	for _, stu := range prep.Students {
		exGrades := make([]model.ExerciseGradeInModule, 0, len(moduleExercises))
		var moduleGrade float64

		for _, ex := range moduleExercises {
			exConfigScore := 100.0
			if sc, ok := prep.Config.Subjects[sub.Name]; ok {
				if mc, ok := sc.Modules[mod.Name]; ok {
					if ec, ok := mc.Exercises[ex.Name]; ok {
						exConfigScore = ec.Score
					}
				}
			}

			pa, hasAssignment := prep.GetAssignment(stu.ID, ex.ID)
			if !hasAssignment {
				continue
			}

			rawGrade, score, review, status, url, redoCount, isChecked := calculateAssignmentGradeInfo(
				&pa, stu.ID, sub.ID, mod.ID, ex.ID, exConfigScore,
			)

			moduleGrade += score

			exGrades = append(exGrades, model.ExerciseGradeInModule{
				ExerciseName:  ex.Name,
				RawGrade:      rawGrade,
				Score:         score,
				Review:        review,
				Status:        status,
				AssignmentURL: url,
				RedoCount:     redoCount,
				IsChecked:     isChecked,
			})
		}

		if moduleGrade > 100.0 {
			moduleGrade = 100.0
		}
		moduleGrade = roundToTwoDecimals(moduleGrade)

		resp.Students = append(resp.Students, model.StudentModuleGrade{
			StudentID:   stu.ID,
			Username:    stu.Username,
			FirstName:   stu.FirstName,
			LastName:    stu.LastName,
			ModuleGrade: moduleGrade,
			Exercises:   exGrades,
		})
	}

	return resp, nil
}

func (c *GradeCalculator) CalculateSubject(req model.SubjectCalcRequest) (*model.SubjectCalcResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	sub, ok := prep.SubjectByName[req.SubjectName]
	if !ok {
		return nil, fmt.Errorf("subject %s not found", req.SubjectName)
	}

	subjectModules := prep.SubjectModules[sub.ID]
	numStudents := len(prep.Students)
	studentsRes := make([]model.StudentSubjectGrade, numStudents)
	subConfig := prep.Config.Subjects[sub.Name]

	if numStudents > 0 {
		var wg sync.WaitGroup
		wg.Add(numStudents)

		for i, stu := range prep.Students {
			go func(idx int, s model.UserEntity) {
				defer wg.Done()

				modGrades := make([]model.ModuleGradeInSubject, 0, len(subjectModules))
				var totalWeightedModuleGrade float64
				var totalModuleWeight float64

				for _, mod := range subjectModules {
					modWeight := 1.0
					if mc, ok := subConfig.Modules[mod.Name]; ok {
						modWeight = mc.Weight
					}

					if modWeight <= 0 {
						continue
					}

					moduleExercises := prep.ModuleExercises[mod.ID]
					var mGrade float64

					for _, ex := range moduleExercises {
						exConfigScore := 100.0
						if mc, ok := subConfig.Modules[mod.Name]; ok {
							if ec, ok := mc.Exercises[ex.Name]; ok {
								exConfigScore = ec.Score
							}
						}

						pa, hasAssignment := prep.GetAssignment(s.ID, ex.ID)
						if !hasAssignment {
							continue
						}

						_, score, _, _, _, _, _ := calculateAssignmentGradeInfo(
							&pa, s.ID, sub.ID, mod.ID, ex.ID, exConfigScore,
						)

						mGrade += score
					}

					if mGrade > 100.0 {
						mGrade = 100.0
					}
					mGrade = roundToTwoDecimals(mGrade)

					totalWeightedModuleGrade += mGrade * modWeight
					totalModuleWeight += modWeight

					modGrades = append(modGrades, model.ModuleGradeInSubject{
						ModuleName:  mod.Name,
						ModuleGrade: mGrade,
					})
				}

				subjectGrade := 0.0
				if totalModuleWeight > 0 {
					subjectGrade = roundToTwoDecimals(totalWeightedModuleGrade / totalModuleWeight)
				}

				studentsRes[idx] = model.StudentSubjectGrade{
					StudentID:    s.ID,
					Username:     s.Username,
					FirstName:    s.FirstName,
					LastName:     s.LastName,
					SubjectGrade: subjectGrade,
					Modules:      modGrades,
				}
			}(i, stu)
		}

		wg.Wait()
	}

	return &model.SubjectCalcResponse{
		SubjectName: sub.Name,
		Students:    studentsRes,
	}, nil
}

func (c *GradeCalculator) CalculateCourse(req model.CourseCalcRequest) (*model.CourseCalcResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	numStudents := len(prep.Students)
	studentsRes := make([]model.StudentCourseGrade, numStudents)

	if numStudents > 0 {
		var wg sync.WaitGroup
		wg.Add(numStudents)

		for i, stu := range prep.Students {
			go func(idx int, s model.UserEntity) {
				defer wg.Done()

				subGrades := make([]model.SubjectGradeInCourse, 0, len(prep.OrderedSubjects))
				var totalWeightedSubjectGrade float64
				var totalSubjectWeight float64

				for _, sub := range prep.OrderedSubjects {
					subWeight := 1.0
					sc, hasSubConfig := prep.Config.Subjects[sub.Name]
					if hasSubConfig {
						subWeight = sc.Weight
					}

					if subWeight <= 0 {
						continue
					}

					subjectModules := prep.SubjectModules[sub.ID]
					var totalWeightedModuleGrade float64
					var totalModuleWeight float64

					for _, mod := range subjectModules {
						modWeight := 1.0
						if hasSubConfig {
							if mc, ok := sc.Modules[mod.Name]; ok {
								modWeight = mc.Weight
							}
						}

						if modWeight <= 0 {
							continue
						}

						moduleExercises := prep.ModuleExercises[mod.ID]
						var mGrade float64

						for _, ex := range moduleExercises {
							exConfigScore := 100.0
							if hasSubConfig {
								if mc, ok := sc.Modules[mod.Name]; ok {
									if ec, ok := mc.Exercises[ex.Name]; ok {
										exConfigScore = ec.Score
									}
								}
							}

							pa, hasAssignment := prep.GetAssignment(s.ID, ex.ID)
							if !hasAssignment {
								continue
							}

							_, score, _, _, _, _, _ := calculateAssignmentGradeInfo(
								&pa, s.ID, sub.ID, mod.ID, ex.ID, exConfigScore,
							)

							mGrade += score
						}

						if mGrade > 100.0 {
							mGrade = 100.0
						}
						mGrade = roundToTwoDecimals(mGrade)

						totalWeightedModuleGrade += mGrade * modWeight
						totalModuleWeight += modWeight
					}

					if totalModuleWeight == 0 {
						continue
					}

					sGrade := roundToTwoDecimals(totalWeightedModuleGrade / totalModuleWeight)
					totalWeightedSubjectGrade += sGrade * subWeight
					totalSubjectWeight += subWeight

					subGrades = append(subGrades, model.SubjectGradeInCourse{
						SubjectName:  sub.Name,
						SubjectGrade: sGrade,
					})
				}

				totalGrade := 0.0
				if totalSubjectWeight > 0 {
					totalGrade = roundToTwoDecimals(totalWeightedSubjectGrade / totalSubjectWeight)
				}

				studentsRes[idx] = model.StudentCourseGrade{
					StudentID:  s.ID,
					Username:   s.Username,
					FirstName:  s.FirstName,
					LastName:   s.LastName,
					TotalGrade: totalGrade,
					Subjects:   subGrades,
				}
			}(i, stu)
		}

		wg.Wait()
	}

	return &model.CourseCalcResponse{
		Students: studentsRes,
	}, nil
}

func (c *GradeCalculator) CalculateStudent(req model.StudentDetailRequest) (*model.StudentDetailResponse, error) {
	prep, err := c.prepareData([]string{req.Username})
	if err != nil {
		return nil, err
	}

	if len(prep.Students) == 0 {
		return nil, fmt.Errorf("student %s not found", req.Username)
	}

	stu := prep.Students[0]

	resp := &model.StudentDetailResponse{
		StudentID: stu.ID,
		Username:  stu.Username,
		FirstName: stu.FirstName,
		LastName:  stu.LastName,
		Subjects:  make([]model.StudentDetailSubjectGrade, 0, len(prep.OrderedSubjects)),
	}

	var totalWeightedSubGrade float64
	var totalSubWeight float64

	for _, sub := range prep.OrderedSubjects {
		subWeight := 1.0
		if sc, ok := prep.Config.Subjects[sub.Name]; ok {
			subWeight = sc.Weight
		}
		if subWeight <= 0 {
			continue
		}

		subjectModules := prep.SubjectModules[sub.ID]
		subModules := make([]model.StudentDetailModuleGrade, 0, len(subjectModules))
		var totalWeightedModGrade float64
		var totalModWeight float64

			for _, mod := range subjectModules {
				modWeight := 1.0
				if sc, ok := prep.Config.Subjects[sub.Name]; ok {
					if mc, ok := sc.Modules[mod.Name]; ok {
						modWeight = mc.Weight
					}
				}
				if modWeight <= 0 {
					continue
				}

				moduleExercises := prep.ModuleExercises[mod.ID]
				exGrades := make([]model.ExerciseGradeInModule, 0, len(moduleExercises))
				var modGrade float64

				for _, ex := range moduleExercises {
					exConfigScore := 100.0
					if sc, ok := prep.Config.Subjects[sub.Name]; ok {
						if mc, ok := sc.Modules[mod.Name]; ok {
							if ec, ok := mc.Exercises[ex.Name]; ok {
								exConfigScore = ec.Score
							}
						}
					}

					pa, hasAssignment := prep.GetAssignment(stu.ID, ex.ID)
					if !hasAssignment {
						continue
					}

					rawGrade, score, review, status, url, redoCount, isChecked := calculateAssignmentGradeInfo(
						&pa, stu.ID, sub.ID, mod.ID, ex.ID, exConfigScore,
					)

					modGrade += score

					exGrades = append(exGrades, model.ExerciseGradeInModule{
						ExerciseName:  ex.Name,
						RawGrade:      rawGrade,
						Score:         score,
						Review:        review,
						Status:        status,
						AssignmentURL: url,
						RedoCount:     redoCount,
						IsChecked:     isChecked,
					})
				}

				if modGrade > 100.0 {
					modGrade = 100.0
				}
				modGrade = roundToTwoDecimals(modGrade)

				totalWeightedModGrade += modGrade * modWeight
				totalModWeight += modWeight

				subModules = append(subModules, model.StudentDetailModuleGrade{
					ModuleName:  mod.Name,
					ModuleGrade: modGrade,
					Exercises:   exGrades,
				})
			}

		if totalModWeight == 0 {
			continue
		}

		subjectGrade := roundToTwoDecimals(totalWeightedModGrade / totalModWeight)
		totalWeightedSubGrade += subjectGrade * subWeight
		totalSubWeight += subWeight

		resp.Subjects = append(resp.Subjects, model.StudentDetailSubjectGrade{
			SubjectName:  sub.Name,
			SubjectGrade: subjectGrade,
			Modules:      subModules,
		})
	}

	if totalSubWeight > 0 {
		resp.TotalGrade = roundToTwoDecimals(totalWeightedSubGrade / totalSubWeight)
	}

	return resp, nil
}

func (c *GradeCalculator) CalculateRecommendations(req model.StudentRecRequest) (*model.StudentRecResponse, error) {
	if req.TargetScore <= 0 {
		req.TargetScore = 70.0
	}

	prep, err := c.prepareData([]string{req.Username})
	if err != nil {
		return nil, err
	}

	if len(prep.Students) == 0 {
		return nil, fmt.Errorf("student %s not found", req.Username)
	}

	stu := prep.Students[0]

	resp := &model.StudentRecResponse{
		Username:    stu.Username,
		TargetScore: req.TargetScore,
		Subjects:    make([]model.SubjectRec, 0, len(prep.OrderedSubjects)),
	}

	for _, sub := range prep.OrderedSubjects {
		subWeight := 1.0
		sc, hasSubConfig := prep.Config.Subjects[sub.Name]
		if hasSubConfig {
			subWeight = sc.Weight
		}
		// Skip zero weight subjects
		if subWeight <= 0 {
			continue
		}

		// Calculate current subject grade directly from prep
		var totalWeightedModuleGrade float64
		var totalModuleWeight float64

		subjectModules := prep.SubjectModules[sub.ID]

		for _, mod := range subjectModules {
			modWeight := 1.0
			if hasSubConfig {
				if mc, ok := sc.Modules[mod.Name]; ok {
					modWeight = mc.Weight
				}
			}
			if modWeight <= 0 {
				continue
			}

			moduleExercises := prep.ModuleExercises[mod.ID]
			var mGrade float64

			for _, ex := range moduleExercises {
				exConfigScore := 100.0
				if hasSubConfig {
					if mc, ok := sc.Modules[mod.Name]; ok {
						if ec, ok := mc.Exercises[ex.Name]; ok {
							exConfigScore = ec.Score
						}
					}
				}
				pa, hasAssignment := prep.GetAssignment(stu.ID, ex.ID)
				if !hasAssignment {
					continue
				}
				_, score, _, _, _, _, _ := calculateAssignmentGradeInfo(
					&pa, stu.ID, sub.ID, mod.ID, ex.ID, exConfigScore,
				)
				mGrade += score
			}

			if mGrade > 100.0 {
				mGrade = 100.0
			}
			mGrade = roundToTwoDecimals(mGrade)

			totalWeightedModuleGrade += mGrade * modWeight
			totalModuleWeight += modWeight
		}

		currentGrade := 0.0
		if totalModuleWeight > 0 {
			currentGrade = roundToTwoDecimals(totalWeightedModuleGrade / totalModuleWeight)
		}

		targetAchieved := currentGrade >= req.TargetScore
		recQueue := make([]model.RecItem, 0)
		projectedGrade := roundToTwoDecimals(currentGrade)

		if !targetAchieved {
			for _, mod := range subjectModules {
				modWeight := 1.0
				if sc, ok := prep.Config.Subjects[sub.Name]; ok {
					if mc, ok := sc.Modules[mod.Name]; ok {
						modWeight = mc.Weight
					}
				}
				if modWeight <= 0 {
					continue
				}

				moduleExercises := prep.ModuleExercises[mod.ID]

				// Sequential recommendations E1 -> E2 -> E3
				prereqMet := true
				for _, ex := range moduleExercises {
					if projectedGrade >= req.TargetScore {
						break
					}

					maxScore := 100.0
					if sc, ok := prep.Config.Subjects[sub.Name]; ok {
						if mc, ok := sc.Modules[mod.Name]; ok {
							if ec, ok := mc.Exercises[ex.Name]; ok {
								maxScore = ec.Score
							}
						}
					}

					pa, hasAssignment := prep.GetAssignment(stu.ID, ex.ID)
					rawGrade, score, _, status, url, _, _ := calculateAssignmentGradeInfo(
						&pa, stu.ID, sub.ID, mod.ID, ex.ID, maxScore,
					)

					if !hasAssignment || rawGrade < 80.0 && status != model.StatusDone {
						if prereqMet {
							recQueue = append(recQueue, model.RecItem{
								SubjectName:   sub.Name,
								ModuleName:    mod.Name,
								ExerciseName:  ex.Name,
								CurrentStatus: status,
								CurrentScore:  roundToTwoDecimals(score),
								MaxScore:      roundToTwoDecimals(maxScore),
								AssignmentURL: url,
							})
							// Assume completing recommended exercise brings score to 100%
							projectedGrade = roundToTwoDecimals(projectedGrade + (maxScore/100.0)*10.0)
							prereqMet = false
						}
					} else {
						prereqMet = true
					}
				}

				if projectedGrade >= req.TargetScore {
					break
				}
			}
		}

		resp.Subjects = append(resp.Subjects, model.SubjectRec{
			SubjectName:      sub.Name,
			CurrentGrade:     roundToTwoDecimals(currentGrade),
			TargetScore:      roundToTwoDecimals(req.TargetScore),
			TargetAchieved:   targetAchieved,
			ProjectedGrade:   roundToTwoDecimals(projectedGrade),
			RecommendedQueue: recQueue,
		})
	}

	return resp, nil
}

func (c *GradeCalculator) CalculateStatusBreakdown(req model.StatusCalcRequest) (*model.CourseStatusResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	courseSummary := make(model.StatusSummary)
	var courseMu sync.Mutex

	subjectList := make([]model.SubjectEntity, 0, len(prep.OrderedSubjects))
	for _, s := range prep.OrderedSubjects {
		subjectList = append(subjectList, s)
	}

	numSubjects := len(subjectList)
	subjectSummaries := make([]model.SubjectStatusSummary, numSubjects)

	if numSubjects > 0 {
		var wg sync.WaitGroup
		wg.Add(numSubjects)

		for sIdx, sub := range subjectList {
			go func(idx int, s model.SubjectEntity) {
				defer wg.Done()

				subWeight := 1.0
				if sc, ok := prep.Config.Subjects[s.Name]; ok {
					subWeight = sc.Weight
				}
				if subWeight <= 0 {
					subjectSummaries[idx] = model.SubjectStatusSummary{
						SubjectName: s.Name,
						Summary:     make(model.StatusSummary),
						Modules:     make([]model.ModuleStatusSummary, 0),
					}
					return
				}

				subSummary := make(model.StatusSummary)
				subjectModules := prep.SubjectModules[s.ID]
				modSummaries := make([]model.ModuleStatusSummary, 0, len(subjectModules))

				for _, mod := range subjectModules {
					modWeight := 1.0
					if sc, ok := prep.Config.Subjects[s.Name]; ok {
						if mc, ok := sc.Modules[mod.Name]; ok {
							modWeight = mc.Weight
						}
					}
					if modWeight <= 0 {
						continue
					}

					modSummary := make(model.StatusSummary)
					moduleExercises := prep.ModuleExercises[mod.ID]
					exSummaries := make([]model.ExerciseStatusSummary, 0, len(moduleExercises))

					for _, ex := range moduleExercises {
						exSummary := make(model.StatusSummary)
						maxScore := 100.0

						for _, stu := range prep.Students {
							pa, hasAssignment := prep.GetAssignment(stu.ID, ex.ID)
							if !hasAssignment {
								continue
							}

							_, _, _, status, _, _, isChecked := calculateAssignmentGradeInfo(
								&pa, stu.ID, s.ID, mod.ID, ex.ID, maxScore,
							)

							exSummary[status]++
							modSummary[status]++
							subSummary[status]++

							if status == model.StatusRedo && !isChecked {
								exSummary[model.StatusRedoUnchecked]++
								modSummary[model.StatusRedoUnchecked]++
								subSummary[model.StatusRedoUnchecked]++
							}
							if status == model.StatusWorkInProgress && !isChecked {
								exSummary[model.StatusWipUnchecked]++
								modSummary[model.StatusWipUnchecked]++
								subSummary[model.StatusWipUnchecked]++
							}
						}

						exSummaries = append(exSummaries, model.ExerciseStatusSummary{
							ExerciseName: ex.Name,
							Summary:      exSummary,
						})
					}

					modSummaries = append(modSummaries, model.ModuleStatusSummary{
						ModuleName: mod.Name,
						Summary:    modSummary,
						Exercises:  exSummaries,
					})
				}

				courseMu.Lock()
				for k, v := range subSummary {
					courseSummary[k] += v
				}
				courseMu.Unlock()

				subjectSummaries[idx] = model.SubjectStatusSummary{
					SubjectName: s.Name,
					Summary:     subSummary,
					Modules:     modSummaries,
				}
			}(sIdx, sub)
		}

		wg.Wait()
	}

	return &model.CourseStatusResponse{
		Summary:  courseSummary,
		Subjects: subjectSummaries,
	}, nil
}

func (c *GradeCalculator) GetStudents() ([]model.StudentResponse, error) {
	cd, err := c.cacheMgr.GetOrFetchData(nil)
	if err != nil {
		return nil, err
	}
	res := make([]model.StudentResponse, 0, len(cd.Students))
	for _, s := range cd.Students {
		res = append(res, model.StudentResponse{
			StudentID: s.ID,
			Username:  s.Username,
			FirstName: s.FirstName,
			LastName:  s.LastName,
		})
	}
	return res, nil
}

func (c *GradeCalculator) CalculateCourseSummary(req model.CourseCalcRequest) (*model.CourseSummaryCalcResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	activeConfig := c.configMgr.GetActiveConfig()
	configSeq := c.configMgr.GetConfigSequence()
	orderedSubjects := configSeq.Subjects

	numStudents := len(prep.Students)
	if numStudents == 0 {
		return &model.CourseSummaryCalcResponse{
			TotalGrade:    0,
			StudentsCount: 0,
			Subjects:      []model.SubjectSummaryGrade{},
		}, nil
	}

	studentDetails := make([]*model.StudentDetailResponse, numStudents)
	for i, stu := range prep.Students {
		resp, err := c.CalculateStudent(model.StudentDetailRequest{Username: stu.Username})
		if err == nil {
			studentDetails[i] = resp
		}
	}

	var totalCourseGradeSum float64
	var validCourseStudentsCount int
	for _, sd := range studentDetails {
		if sd != nil {
			totalCourseGradeSum += sd.TotalGrade
			validCourseStudentsCount++
		}
	}
	avgCourseGrade := 0.0
	if validCourseStudentsCount > 0 {
		avgCourseGrade = roundToTwoDecimals(totalCourseGradeSum / float64(validCourseStudentsCount))
	}

	subjectSummaries := make([]model.SubjectSummaryGrade, 0, len(orderedSubjects))

	for _, subKey := range orderedSubjects {
		subCfg, exists := activeConfig.Subjects[subKey]
		if !exists {
			continue
		}

		orderedModules := configSeq.Modules[subKey]
		moduleSummaries := make([]model.ModuleSummaryGrade, 0, len(orderedModules))

		var subjectGradeSum float64
		var subjectStudentCount int

		for _, sd := range studentDetails {
			if sd == nil {
				continue
			}
			for _, sub := range sd.Subjects {
				if sub.SubjectName == subKey {
					subjectGradeSum += sub.SubjectGrade
					subjectStudentCount++
					break
				}
			}
		}

		avgSubjectGrade := 0.0
		if subjectStudentCount > 0 {
			avgSubjectGrade = roundToTwoDecimals(subjectGradeSum / float64(subjectStudentCount))
		}

		for _, modKey := range orderedModules {
			modCfg, modExists := subCfg.Modules[modKey]
			if !modExists {
				continue
			}

			orderedExercises := configSeq.Exercises[modKey]
			exerciseSummaries := make([]model.ExerciseSummaryGrade, 0, len(orderedExercises))

			var moduleGradeSum float64
			var moduleStudentCount int

			for _, sd := range studentDetails {
				if sd == nil {
					continue
				}
				for _, sub := range sd.Subjects {
					if sub.SubjectName == subKey {
						for _, mod := range sub.Modules {
							if mod.ModuleName == modKey {
								moduleGradeSum += mod.ModuleGrade
								moduleStudentCount++
								break
							}
						}
					}
				}
			}

			avgModuleGrade := 0.0
			if moduleStudentCount > 0 {
				avgModuleGrade = roundToTwoDecimals(moduleGradeSum / float64(moduleStudentCount))
			}

			for _, exKey := range orderedExercises {
				exCfg, exExists := modCfg.Exercises[exKey]
				if !exExists {
					continue
				}

				var exerciseScoreSum float64
				var exerciseStudentCount int

				for _, sd := range studentDetails {
					if sd == nil {
						continue
					}
					for _, sub := range sd.Subjects {
						if sub.SubjectName == subKey {
							for _, mod := range sub.Modules {
								if mod.ModuleName == modKey {
									for _, ex := range mod.Exercises {
										if ex.ExerciseName == exKey {
											exerciseScoreSum += ex.Score
											exerciseStudentCount++
											break
										}
									}
								}
							}
						}
					}
				}

				avgExerciseScore := 0.0
				if exerciseStudentCount > 0 {
					avgExerciseScore = roundToTwoDecimals(exerciseScoreSum / float64(exerciseStudentCount))
				}

				exerciseSummaries = append(exerciseSummaries, model.ExerciseSummaryGrade{
					ExerciseName:  exKey,
					TotalGrade:    avgExerciseScore,
					MaxScore:      exCfg.Score,
					StudentsCount: exerciseStudentCount,
				})
			}

			moduleSummaries = append(moduleSummaries, model.ModuleSummaryGrade{
				ModuleName:    modKey,
				TotalGrade:    avgModuleGrade,
				Weight:        modCfg.Weight,
				StudentsCount: moduleStudentCount,
				Exercises:     exerciseSummaries,
			})
		}

		subjectSummaries = append(subjectSummaries, model.SubjectSummaryGrade{
			SubjectName:   subKey,
			TotalGrade:    avgSubjectGrade,
			Weight:        subCfg.Weight,
			StudentsCount: subjectStudentCount,
			Modules:       moduleSummaries,
		})
	}

	return &model.CourseSummaryCalcResponse{
		TotalGrade:    avgCourseGrade,
		StudentsCount: validCourseStudentsCount,
		Subjects:      subjectSummaries,
	}, nil
}
