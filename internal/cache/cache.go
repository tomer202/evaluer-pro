package cache

import (
	"sync"
	"time"

	"evaluer/internal/dal"
	"evaluer/internal/model"
)

type CachedData struct {
	Subjects    []model.SubjectEntity
	Modules     []model.ModuleEntity
	Exercises   []model.ExerciseEntity
	Students    []model.UserEntity
	Assignments []model.AssignmentEntity
	Responses   []model.ResponseEntity
	FetchedAt   time.Time
}

type CacheManager struct {
	mu           sync.RWMutex
	dal          *dal.DAL
	testOverride *CachedData
}

func NewCacheManager(dalInstance *dal.DAL) *CacheManager {
	return &CacheManager{
		dal: dalInstance,
	}
}

func (cm *CacheManager) SetCachedData(data *CachedData) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.testOverride = data
}

func (cm *CacheManager) GetOrFetchData(targetStudents []string) (*CachedData, error) {
	cm.mu.RLock()
	if cm.testOverride != nil {
		data := cm.testOverride
		cm.mu.RUnlock()
		return data, nil
	}
	cm.mu.RUnlock()

	// Always execute real-time DB queries on every request
	return cm.Refresh(targetStudents)
}

func (cm *CacheManager) Refresh(targetStudents []string) (*CachedData, error) {
	var targetUserIDs []int

	if cm.dal != nil && len(targetStudents) > 0 {
		stus, err := cm.dal.FetchStudents(targetStudents)
		if err != nil {
			return nil, err
		}
		for _, s := range stus {
			targetUserIDs = append(targetUserIDs, s.ID)
		}
	}

	var wg sync.WaitGroup
	var subjects []model.SubjectEntity
	var modules []model.ModuleEntity
	var exercises []model.ExerciseEntity
	var students []model.UserEntity
	var assignments []model.AssignmentEntity
	var responses []model.ResponseEntity

	var errSub, errMod, errExe, errStu, errAss, errRes error

	wg.Add(6)

	go func() {
		defer wg.Done()
		if cm.dal != nil {
			subjects, errSub = cm.dal.FetchSubjects()
		}
	}()

	go func() {
		defer wg.Done()
		if cm.dal != nil {
			modules, errMod = cm.dal.FetchModules()
		}
	}()

	go func() {
		defer wg.Done()
		if cm.dal != nil {
			exercises, errExe = cm.dal.FetchExercises()
		}
	}()

	go func() {
		defer wg.Done()
		if cm.dal != nil {
			students, errStu = cm.dal.FetchStudents(nil)
		}
	}()

	go func() {
		defer wg.Done()
		if cm.dal != nil {
			assignments, errAss = cm.dal.FetchAssignments(targetUserIDs)
		}
	}()

	go func() {
		defer wg.Done()
		if cm.dal != nil {
			responses, errRes = cm.dal.FetchResponses(targetUserIDs)
		}
	}()

	wg.Wait()

	if errSub != nil {
		return nil, errSub
	}
	if errMod != nil {
		return nil, errMod
	}
	if errExe != nil {
		return nil, errExe
	}
	if errStu != nil {
		return nil, errStu
	}
	if errAss != nil {
		return nil, errAss
	}
	if errRes != nil {
		return nil, errRes
	}

	newData := &CachedData{
		Subjects:    subjects,
		Modules:     modules,
		Exercises:   exercises,
		Students:    students,
		Assignments: assignments,
		Responses:   responses,
		FetchedAt:   time.Now(),
	}

	return newData, nil
}
