package service

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"evaluer/internal/cache"
	"evaluer/internal/config"
	"evaluer/internal/model"
)

var numberRegex = regexp.MustCompile(`\b([0-9]+(?:\.[0-9]+)?)\b`)

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

func getHiveBaseURL() string {
	url := os.Getenv("HIVE_BASE_URL")
	if url == "" {
		url = "https://20.13.2.62"
	}
	return strings.TrimRight(url, "/")
}

// ParseGradeFromContent extracts numerical grade from segel review content string
func ParseGradeFromContent(content string) float64 {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0.0
	}

	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		matches := numberRegex.FindAllString(line, -1)
		if len(matches) > 0 {
			lastMatch := matches[len(matches)-1]
			if val, err := strconv.ParseFloat(lastMatch, 64); err == nil {
				if val >= 0 && val <= 100 {
					return val
				}
			}
		}
	}
	return 0.0
}

type PreparedAssignment struct {
	Assignment model.AssignmentEntity
	Responses  []model.ResponseEntity
}

type PreparedData struct {
	Config           model.CourseConfig
	Students         []model.UserEntity
	Subjects         map[int]model.SubjectEntity
	Modules          map[int]model.ModuleEntity
	Exercises        map[int]model.ExerciseEntity
	SubjectByName    map[string]model.SubjectEntity
	ModuleByName     map[string]model.ModuleEntity
	ExerciseByName   map[string]model.ExerciseEntity
	// Key: UserID -> ExerciseID -> PreparedAssignment
	UserAssignments map[int]map[int]PreparedAssignment
}

func (c *GradeCalculator) prepareData(targetStudents []string) (*PreparedData, error) {
	cd, err := c.cacheMgr.GetOrFetchData(targetStudents)
	if err != nil {
		return nil, err
	}

	cfg := c.configMgr.GetActiveConfig()

	prep := &PreparedData{
		Config:          cfg,
		Students:        cd.Students,
		Subjects:        make(map[int]model.SubjectEntity),
		Modules:         make(map[int]model.ModuleEntity),
		Exercises:       make(map[int]model.ExerciseEntity),
		SubjectByName:   make(map[string]model.SubjectEntity),
		ModuleByName:    make(map[string]model.ModuleEntity),
		ExerciseByName:  make(map[string]model.ExerciseEntity),
		UserAssignments: make(map[int]map[int]PreparedAssignment),
	}

	for _, s := range cd.Subjects {
		prep.Subjects[s.ID] = s
		prep.SubjectByName[s.Name] = s
	}

	for _, m := range cd.Modules {
		prep.Modules[m.ID] = m
		prep.ModuleByName[m.Name] = m
	}

	for _, e := range cd.Exercises {
		prep.Exercises[e.ID] = e
		prep.ExerciseByName[e.Name] = e
	}

	// Map responses by ThreadID
	respMap := make(map[int][]model.ResponseEntity)
	for _, r := range cd.Responses {
		respMap[r.ThreadID] = append(respMap[r.ThreadID], r)
	}

	// Map assignments by UserID -> ExerciseID
	for _, a := range cd.Assignments {
		if prep.UserAssignments[a.UserID] == nil {
			prep.UserAssignments[a.UserID] = make(map[int]PreparedAssignment)
		}
		prep.UserAssignments[a.UserID][a.ExerciseID] = PreparedAssignment{
			Assignment: a,
			Responses:  respMap[a.ID],
		}
	}

	return prep, nil
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
			}
		} else {
			if r.Date.After(latestStudentDate) {
				latestStudentDate = r.Date
			}
		}
	}

	if hasSegelReview {
		if latestStudentDate.IsZero() {
			isChecked = true
		} else {
			isChecked = !latestStudentDate.After(latestSegelDate)
		}
	} else {
		// If status is DONE or NEW, default is checked unless WIP/REDO/SUBMITTED unreviewed
		if assign.AssignmentStatus == model.AssignmentStatusDone {
			isChecked = true
		} else if assign.AssignmentStatus == model.AssignmentStatusNew {
			isChecked = true
		} else {
			isChecked = false
		}
	}

	switch assign.AssignmentStatus {
	case model.AssignmentStatusDone:
		rawGrade = 100.0
	case model.AssignmentStatusRedo, model.AssignmentStatusWorkInProgress:
		// Extract grade from segel response content
		if review != "" {
			rawGrade = ParseGradeFromContent(review)
		} else {
			rawGrade = 0.0
		}
	default:
		rawGrade = 0.0
	}

	score = (rawGrade / 100.0) * maxScore
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
		userAssMap := prep.UserAssignments[stu.ID]

		exGrades := make([]model.ExerciseGradeInModule, 0)
		var totalWeightedScore float64
		var totalMaxScore float64

		for _, ex := range moduleExercises {
			maxScore := 100.0
			if sc, ok := prep.Config.Subjects[sub.Name]; ok {
				if mc, ok := sc.Modules[mod.Name]; ok {
					if ec, ok := mc.Exercises[ex.Name]; ok {
						maxScore = ec.Score
					}
				}
			}

			pa, hasAssignment := userAssMap[ex.ID]
			// Unopened exercise filtering rule: omit exercises student hasn't opened
			if !hasAssignment {
				continue
			}

			rawGrade, score, review, status, url, redoCount, isChecked := calculateAssignmentGradeInfo(
				&pa, stu.ID, sub.ID, mod.ID, ex.ID, maxScore,
			)

			totalWeightedScore += score
			totalMaxScore += maxScore

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

		moduleGrade := 100.0
		if totalMaxScore > 0 {
			moduleGrade = (totalWeightedScore / totalMaxScore) * 100.0
		} else if len(exGrades) == 0 {
			moduleGrade = 0.0
		}

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

	var subjectModules []model.ModuleEntity
	for _, m := range prep.Modules {
		if m.ParentSubjectID == sub.ID {
			subjectModules = append(subjectModules, m)
		}
	}

	resp := &model.SubjectCalcResponse{
		SubjectName: sub.Name,
		Students:    make([]model.StudentSubjectGrade, 0),
	}

	subConfig := prep.Config.Subjects[sub.Name]

	for _, stu := range prep.Students {
		modGrades := make([]model.ModuleGradeInSubject, 0)
		var totalWeightedModuleGrade float64
		var totalModuleWeight float64

		for _, mod := range subjectModules {
			modWeight := 1.0
			if mc, ok := subConfig.Modules[mod.Name]; ok {
				modWeight = mc.Weight
			}

			// Exclude zero-weight modules
			if modWeight <= 0 {
				continue
			}

			mResp, err := c.CalculateModule(model.ModuleCalcRequest{
				SubjectName: sub.Name,
				ModuleName:  mod.Name,
				Students:    []string{stu.Username},
			})

			mGrade := 0.0
			if err == nil && len(mResp.Students) > 0 {
				mGrade = mResp.Students[0].ModuleGrade
			}

			totalWeightedModuleGrade += mGrade * modWeight
			totalModuleWeight += modWeight

			modGrades = append(modGrades, model.ModuleGradeInSubject{
				ModuleName:  mod.Name,
				ModuleGrade: mGrade,
			})
		}

		subjectGrade := 0.0
		if totalModuleWeight > 0 {
			subjectGrade = totalWeightedModuleGrade / totalModuleWeight
		}

		resp.Students = append(resp.Students, model.StudentSubjectGrade{
			StudentID:    stu.ID,
			Username:     stu.Username,
			FirstName:    stu.FirstName,
			LastName:     stu.LastName,
			SubjectGrade: subjectGrade,
			Modules:      modGrades,
		})
	}

	return resp, nil
}

func (c *GradeCalculator) CalculateCourse(req model.CourseCalcRequest) (*model.CourseCalcResponse, error) {
	prep, err := c.prepareData(req.Students)
	if err != nil {
		return nil, err
	}

	resp := &model.CourseCalcResponse{
		Students: make([]model.StudentCourseGrade, 0),
	}

	for _, stu := range prep.Students {
		subGrades := make([]model.SubjectGradeInCourse, 0)
		var totalWeightedSubjectGrade float64
		var totalSubjectWeight float64

		for _, sub := range prep.Subjects {
			subWeight := 1.0
			if sc, ok := prep.Config.Subjects[sub.Name]; ok {
				subWeight = sc.Weight
			}

			if subWeight <= 0 {
				continue
			}

			sResp, err := c.CalculateSubject(model.SubjectCalcRequest{
				SubjectName: sub.Name,
				Students:    []string{stu.Username},
			})

			sGrade := 0.0
			if err == nil && len(sResp.Students) > 0 {
				sGrade = sResp.Students[0].SubjectGrade
			}

			totalWeightedSubjectGrade += sGrade * subWeight
			totalSubjectWeight += subWeight

			subGrades = append(subGrades, model.SubjectGradeInCourse{
				SubjectName:  sub.Name,
				SubjectGrade: sGrade,
			})
		}

		totalGrade := 0.0
		if totalSubjectWeight > 0 {
			totalGrade = totalWeightedSubjectGrade / totalSubjectWeight
		}

		resp.Students = append(resp.Students, model.StudentCourseGrade{
			StudentID:  stu.ID,
			Username:   stu.Username,
			FirstName:  stu.FirstName,
			LastName:   stu.LastName,
			TotalGrade: totalGrade,
			Subjects:   subGrades,
		})
	}

	return resp, nil
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
	userAssMap := prep.UserAssignments[stu.ID]

	resp := &model.StudentDetailResponse{
		StudentID: stu.ID,
		Username:  stu.Username,
		FirstName: stu.FirstName,
		LastName:  stu.LastName,
		Subjects:  make([]model.StudentDetailSubjectGrade, 0),
	}

	var totalWeightedSubGrade float64
	var totalSubWeight float64

	for _, sub := range prep.Subjects {
		subWeight := 1.0
		if sc, ok := prep.Config.Subjects[sub.Name]; ok {
			subWeight = sc.Weight
		}
		if subWeight <= 0 {
			continue
		}

		var subjectModules []model.ModuleEntity
		for _, m := range prep.Modules {
			if m.ParentSubjectID == sub.ID {
				subjectModules = append(subjectModules, m)
			}
		}

		subModules := make([]model.StudentDetailModuleGrade, 0)
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

			var moduleExercises []model.ExerciseEntity
			for _, ex := range prep.Exercises {
				if ex.ParentModuleID == mod.ID {
					moduleExercises = append(moduleExercises, ex)
				}
			}

			exGrades := make([]model.ExerciseGradeInModule, 0)
			var totalWeightedExScore float64
			var totalExMaxScore float64

			for _, ex := range moduleExercises {
				maxScore := 100.0
				if sc, ok := prep.Config.Subjects[sub.Name]; ok {
					if mc, ok := sc.Modules[mod.Name]; ok {
						if ec, ok := mc.Exercises[ex.Name]; ok {
							maxScore = ec.Score
						}
					}
				}

				pa, hasAssignment := userAssMap[ex.ID]
				if !hasAssignment {
					continue
				}

				rawGrade, score, review, status, url, redoCount, isChecked := calculateAssignmentGradeInfo(
					&pa, stu.ID, sub.ID, mod.ID, ex.ID, maxScore,
				)

				totalWeightedExScore += score
				totalExMaxScore += maxScore

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

			modGrade := 100.0
			if totalExMaxScore > 0 {
				modGrade = (totalWeightedExScore / totalExMaxScore) * 100.0
			} else if len(exGrades) == 0 {
				modGrade = 0.0
			}

			totalWeightedModGrade += modGrade * modWeight
			totalModWeight += modWeight

			subModules = append(subModules, model.StudentDetailModuleGrade{
				ModuleName:  mod.Name,
				ModuleGrade: modGrade,
				Exercises:   exGrades,
			})
		}

		subjectGrade := 0.0
		if totalModWeight > 0 {
			subjectGrade = totalWeightedModGrade / totalModWeight
		}

		totalWeightedSubGrade += subjectGrade * subWeight
		totalSubWeight += subWeight

		resp.Subjects = append(resp.Subjects, model.StudentDetailSubjectGrade{
			SubjectName:  sub.Name,
			SubjectGrade: subjectGrade,
			Modules:      subModules,
		})
	}

	if totalSubWeight > 0 {
		resp.TotalGrade = totalWeightedSubGrade / totalSubWeight
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
	userAssMap := prep.UserAssignments[stu.ID]

	resp := &model.StudentRecResponse{
		Username:    stu.Username,
		TargetScore: req.TargetScore,
		Subjects:    make([]model.SubjectRec, 0),
	}

	for _, sub := range prep.Subjects {
		subWeight := 1.0
		if sc, ok := prep.Config.Subjects[sub.Name]; ok {
			subWeight = sc.Weight
		}
		// Skip zero weight subjects
		if subWeight <= 0 {
			continue
		}

		// Calculate current subject grade
		sResp, _ := c.CalculateSubject(model.SubjectCalcRequest{
			SubjectName: sub.Name,
			Students:    []string{stu.Username},
		})

		currentGrade := 0.0
		if sResp != nil && len(sResp.Students) > 0 {
			currentGrade = sResp.Students[0].SubjectGrade
		}

		targetAchieved := currentGrade >= req.TargetScore
		recQueue := make([]model.RecItem, 0)
		projectedGrade := currentGrade

		if !targetAchieved {
			var subjectModules []model.ModuleEntity
			for _, m := range prep.Modules {
				if m.ParentSubjectID == sub.ID {
					subjectModules = append(subjectModules, m)
				}
			}

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

				var moduleExercises []model.ExerciseEntity
				for _, ex := range prep.Exercises {
					if ex.ParentModuleID == mod.ID {
						moduleExercises = append(moduleExercises, ex)
					}
				}

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

					pa, hasAssignment := userAssMap[ex.ID]
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
								CurrentScore:  score,
								MaxScore:      maxScore,
								AssignmentURL: url,
							})
							// Assume completing recommended exercise brings score to 100%
							projectedGrade += (maxScore / 100.0) * 10.0 // Incremental boost
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
			CurrentGrade:     currentGrade,
			TargetScore:      req.TargetScore,
			TargetAchieved:   targetAchieved,
			ProjectedGrade:   projectedGrade,
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
	subjectSummaries := make([]model.SubjectStatusSummary, 0)

	for _, sub := range prep.Subjects {
		subSummary := make(model.StatusSummary)
		var subjectModules []model.ModuleEntity
		for _, m := range prep.Modules {
			if m.ParentSubjectID == sub.ID {
				subjectModules = append(subjectModules, m)
			}
		}

		modSummaries := make([]model.ModuleStatusSummary, 0)

		for _, mod := range subjectModules {
			modSummary := make(model.StatusSummary)
			var moduleExercises []model.ExerciseEntity
			for _, ex := range prep.Exercises {
				if ex.ParentModuleID == mod.ID {
					moduleExercises = append(moduleExercises, ex)
				}
			}

			exSummaries := make([]model.ExerciseStatusSummary, 0)

			for _, ex := range moduleExercises {
				exSummary := make(model.StatusSummary)
				maxScore := 100.0

				for _, stu := range prep.Students {
					userAssMap := prep.UserAssignments[stu.ID]
					pa, hasAssignment := userAssMap[ex.ID]

					var status string
					var isChecked bool

					if !hasAssignment {
						status = model.StatusNew
						isChecked = true
					} else {
						_, _, _, status, _, _, isChecked = calculateAssignmentGradeInfo(
							&pa, stu.ID, sub.ID, mod.ID, ex.ID, maxScore,
						)
					}

					exSummary[status]++
					modSummary[status]++
					subSummary[status]++
					courseSummary[status]++

					if status == model.StatusRedo && !isChecked {
						exSummary[model.StatusRedoUnchecked]++
						modSummary[model.StatusRedoUnchecked]++
						subSummary[model.StatusRedoUnchecked]++
						courseSummary[model.StatusRedoUnchecked]++
					}
					if status == model.StatusWorkInProgress && !isChecked {
						exSummary[model.StatusWipUnchecked]++
						modSummary[model.StatusWipUnchecked]++
						subSummary[model.StatusWipUnchecked]++
						courseSummary[model.StatusWipUnchecked]++
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

		subjectSummaries = append(subjectSummaries, model.SubjectStatusSummary{
			SubjectName: sub.Name,
			Summary:     subSummary,
			Modules:     modSummaries,
		})
	}

	return &model.CourseStatusResponse{
		Summary:  courseSummary,
		Subjects: subjectSummaries,
	}, nil
}
