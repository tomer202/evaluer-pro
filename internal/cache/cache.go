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
	mu        sync.RWMutex
	dal       *dal.DAL
	cache     *CachedData
	ttl       time.Duration
	isEnabled bool
}

func NewCacheManager(dalInstance *dal.DAL) *CacheManager {
	return &CacheManager{
		dal:       dalInstance,
		ttl:       24 * time.Hour,
		isEnabled: true,
	}
}

func (cm *CacheManager) GetOrFetchData(targetStudents []string) (*CachedData, error) {
	cm.mu.RLock()
	if cm.isEnabled && cm.cache != nil && time.Since(cm.cache.FetchedAt) < cm.ttl {
		cached := cm.cache
		cm.mu.RUnlock()
		return cached, nil
	}
	cm.mu.RUnlock()

	// High concurrency parallel prefetching
	return cm.Refresh(targetStudents)
}

func (cm *CacheManager) Refresh(targetStudents []string) (*CachedData, error) {
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
		subjects, errSub = cm.dal.FetchSubjects()
	}()

	go func() {
		defer wg.Done()
		modules, errMod = cm.dal.FetchModules()
	}()

	go func() {
		defer wg.Done()
		exercises, errExe = cm.dal.FetchExercises()
	}()

	go func() {
		defer wg.Done()
		students, errStu = cm.dal.FetchStudents(targetStudents)
	}()

	go func() {
		defer wg.Done()
		assignments, errAss = cm.dal.FetchAssignments()
	}()

	go func() {
		defer wg.Done()
		responses, errRes = cm.dal.FetchResponses()
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

	cm.mu.Lock()
	cm.cache = newData
	cm.mu.Unlock()

	return newData, nil
}
