package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"evaluer/internal/dal"
	"evaluer/internal/model"

	"gopkg.in/yaml.v3"
)

type ConfigManager struct {
	mu         sync.RWMutex
	configPath string
	active     model.CourseConfig
	dal        *dal.DAL
}

func NewConfigManager(dalInstance *dal.DAL) *ConfigManager {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = ".local/config.yaml"
	}
	cm := &ConfigManager{
		configPath: path,
		dal:        dalInstance,
	}
	// Try loading initial config, otherwise generate default
	if err := cm.Load(); err != nil {
		cm.GenerateAndSave(path)
	}
	return cm
}

func (cm *ConfigManager) GetConfigPath() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configPath
}

func (cm *ConfigManager) GetActiveConfig() model.CourseConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.active
}

func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	var cfg model.CourseConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.Subjects == nil {
		cfg.Subjects = make(map[string]model.SubjectConfig)
	}
	cm.active = cfg
	return nil
}

func (cm *ConfigManager) Save(path string, cfg model.CourseConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if path == "" {
		path = cm.configPath
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	cm.configPath = path
	cm.active = cfg
	return nil
}

func (cm *ConfigManager) GenerateSkeleton() (model.CourseConfig, error) {
	subjects, err := cm.dal.FetchSubjects()
	if err != nil {
		return model.CourseConfig{}, err
	}
	modules, err := cm.dal.FetchModules()
	if err != nil {
		return model.CourseConfig{}, err
	}
	exercises, err := cm.dal.FetchExercises()
	if err != nil {
		return model.CourseConfig{}, err
	}

	modMap := make(map[int][]model.ExerciseEntity)
	for _, e := range exercises {
		modMap[e.ParentModuleID] = append(modMap[e.ParentModuleID], e)
	}

	subMap := make(map[int][]model.ModuleEntity)
	for _, m := range modules {
		subMap[m.ParentSubjectID] = append(subMap[m.ParentSubjectID], m)
	}

	cfg := model.CourseConfig{
		Subjects: make(map[string]model.SubjectConfig),
	}

	for _, s := range subjects {
		sc := model.SubjectConfig{
			Name:    s.Name,
			Weight:  1.0,
			Modules: make(map[string]model.ModuleConfig),
		}
		for _, m := range subMap[s.ID] {
			mc := model.ModuleConfig{
				Name:      m.Name,
				Weight:    1.0,
				Exercises: make(map[string]model.ExerciseConfig),
			}
			for _, e := range modMap[m.ID] {
				mc.Exercises[e.Name] = model.ExerciseConfig{
					Name:  e.Name,
					Score: 100.0,
				}
			}
			sc.Modules[m.Name] = mc
		}
		cfg.Subjects[s.Name] = sc
	}

	return cfg, nil
}

func (cm *ConfigManager) GenerateAndSave(path string) (model.CourseConfig, error) {
	cfg, err := cm.GenerateSkeleton()
	if err != nil {
		return model.CourseConfig{}, err
	}
	if path == "" {
		path = cm.configPath
	}
	if err := cm.Save(path, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (cm *ConfigManager) Patch(patchData map[string]interface{}) (model.CourseConfig, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	active := cm.active

	if subjectsRaw, ok := patchData["subjects"].(map[string]interface{}); ok {
		for sName, sVal := range subjectsRaw {
			sObj, ok := sVal.(map[string]interface{})
			if !ok {
				continue
			}
			sub, exists := active.Subjects[sName]
			if !exists {
				sub = model.SubjectConfig{Name: sName, Weight: 1.0, Modules: make(map[string]model.ModuleConfig)}
			}
			if w, ok := sObj["weight"].(float64); ok {
				sub.Weight = w
			}
			if modsRaw, ok := sObj["modules"].(map[string]interface{}); ok {
				if sub.Modules == nil {
					sub.Modules = make(map[string]model.ModuleConfig)
				}
				for mName, mVal := range modsRaw {
					mObj, ok := mVal.(map[string]interface{})
					if !ok {
						continue
					}
					mod, mExists := sub.Modules[mName]
					if !mExists {
						mod = model.ModuleConfig{Name: mName, Weight: 1.0, Exercises: make(map[string]model.ExerciseConfig)}
					}
					if w, ok := mObj["weight"].(float64); ok {
						mod.Weight = w
					}
					if exesRaw, ok := mObj["exercises"].(map[string]interface{}); ok {
						if mod.Exercises == nil {
							mod.Exercises = make(map[string]model.ExerciseConfig)
						}
						for eName, eVal := range exesRaw {
							eObj, ok := eVal.(map[string]interface{})
							if !ok {
								continue
							}
							ex, eExists := mod.Exercises[eName]
							if !eExists {
								ex = model.ExerciseConfig{Name: eName, Score: 100.0}
							}
							if s, ok := eObj["score"].(float64); ok {
								ex.Score = s
							}
							mod.Exercises[eName] = ex
						}
					}
					sub.Modules[mName] = mod
				}
			}
			active.Subjects[sName] = sub
		}
	}

	cm.active = active
	_ = cm.Save(cm.configPath, cm.active)
	return cm.active, fmt.Errorf("") // nil error
}
