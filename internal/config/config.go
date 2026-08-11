package config

import (
	"bytes"
	"encoding/json"
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
	rawYAML    []byte
	dal        *dal.DAL
}

type OrderedConfigSequence struct {
	Subjects  []string
	Modules   map[string][]string // SubjectName -> []ModuleName
	Exercises map[string][]string // ModuleName -> []ExerciseName
}

func (cm *ConfigManager) GetConfigSequence() OrderedConfigSequence {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	seq := OrderedConfigSequence{
		Subjects:  make([]string, 0),
		Modules:   make(map[string][]string),
		Exercises: make(map[string][]string),
	}

	var yamlData []byte
	if len(cm.rawYAML) > 0 {
		yamlData = cm.rawYAML
	} else {
		yamlData, _ = yaml.Marshal(cm.active)
	}

	if len(yamlData) == 0 {
		return seq
	}

	var node yaml.Node
	if err := yaml.Unmarshal(yamlData, &node); err != nil {
		return seq
	}

	var rootMap *yaml.Node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		rootMap = node.Content[0]
	} else if node.Kind == yaml.MappingNode {
		rootMap = &node
	}

	if rootMap == nil || rootMap.Kind != yaml.MappingNode {
		return seq
	}

	for i := 0; i < len(rootMap.Content); i += 2 {
		if rootMap.Content[i].Value == "subjects" && rootMap.Content[i+1].Kind == yaml.MappingNode {
			subsMap := rootMap.Content[i+1]
			for sIdx := 0; sIdx < len(subsMap.Content); sIdx += 2 {
				subName := subsMap.Content[sIdx].Value
				seq.Subjects = append(seq.Subjects, subName)
				subNode := subsMap.Content[sIdx+1]

				if subNode.Kind == yaml.MappingNode {
					for mKeyIdx := 0; mKeyIdx < len(subNode.Content); mKeyIdx += 2 {
						if subNode.Content[mKeyIdx].Value == "modules" && subNode.Content[mKeyIdx+1].Kind == yaml.MappingNode {
							modsMap := subNode.Content[mKeyIdx+1]
							modList := make([]string, 0)
							for mIdx := 0; mIdx < len(modsMap.Content); mIdx += 2 {
								modName := modsMap.Content[mIdx].Value
								modList = append(modList, modName)
								modNode := modsMap.Content[mIdx+1]

								if modNode.Kind == yaml.MappingNode {
									for eKeyIdx := 0; eKeyIdx < len(modNode.Content); eKeyIdx += 2 {
										if modNode.Content[eKeyIdx].Value == "exercises" && modNode.Content[eKeyIdx+1].Kind == yaml.MappingNode {
											exesMap := modNode.Content[eKeyIdx+1]
											exList := make([]string, 0)
											for eIdx := 0; eIdx < len(exesMap.Content); eIdx += 2 {
												exName := exesMap.Content[eIdx].Value
												exList = append(exList, exName)
											}
											seq.Exercises[modName] = exList
										}
									}
								}
							}
							seq.Modules[subName] = modList
						}
					}
				}
			}
		}
	}

	return seq
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
	if err := cm.Load(); err != nil && dalInstance != nil {
		_, _ = cm.GenerateAndSaveRaw(path)
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

func (cm *ConfigManager) SetActiveConfig(cfg model.CourseConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.active = cfg
	data, err := yaml.Marshal(cfg)
	if err == nil {
		cm.rawYAML = data
	}
}

func (cm *ConfigManager) GetActiveConfigRawJSON() ([]byte, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.rawYAML) > 0 {
		return ConvertYAMLToJSON(cm.rawYAML)
	}
	data, err := yaml.Marshal(cm.active)
	if err != nil {
		return nil, err
	}
	return ConvertYAMLToJSON(data)
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
	cm.rawYAML = data
	return nil
}

func stripFlowStyle(n *yaml.Node) {
	if n == nil {
		return
	}
	if n.Style == yaml.FlowStyle {
		n.Style = 0
	}
	for _, child := range n.Content {
		stripFlowStyle(child)
	}
}

func encodeNodeToJSON(n *yaml.Node) ([]byte, error) {
	if n == nil {
		return []byte("null"), nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) > 0 {
			return encodeNodeToJSON(n.Content[0])
		}
		return []byte("null"), nil

	case yaml.MappingNode:
		var buf bytes.Buffer
		buf.WriteString("{")
		for i := 0; i < len(n.Content); i += 2 {
			if i > 0 {
				buf.WriteString(",")
			}
			keyBytes, err := json.Marshal(n.Content[i].Value)
			if err != nil {
				return nil, err
			}
			valBytes, err := encodeNodeToJSON(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			buf.Write(keyBytes)
			buf.WriteString(":")
			buf.Write(valBytes)
		}
		buf.WriteString("}")
		return buf.Bytes(), nil

	case yaml.SequenceNode:
		var buf bytes.Buffer
		buf.WriteString("[")
		for i, item := range n.Content {
			if i > 0 {
				buf.WriteString(",")
			}
			itemBytes, err := encodeNodeToJSON(item)
			if err != nil {
				return nil, err
			}
			buf.Write(itemBytes)
		}
		buf.WriteString("]")
		return buf.Bytes(), nil

	case yaml.ScalarNode:
		if n.Tag == "!!null" || n.Value == "null" || (n.Value == "" && n.Style == 0) {
			if n.Tag == "!!str" {
				return json.Marshal("")
			}
			return []byte("null"), nil
		}
		if n.Tag == "!!bool" {
			if n.Value == "true" || n.Value == "false" {
				return []byte(n.Value), nil
			}
		}
		if n.Tag == "!!int" || n.Tag == "!!float" {
			return []byte(n.Value), nil
		}
		var rawVal interface{}
		if err := n.Decode(&rawVal); err == nil {
			return json.Marshal(rawVal)
		}
		return json.Marshal(n.Value)

	case yaml.AliasNode:
		return encodeNodeToJSON(n.Alias)

	default:
		return json.Marshal(n.Value)
	}
}

// ConvertYAMLToJSON converts raw YAML bytes to JSON bytes while preserving exact key & value sequence.
func ConvertYAMLToJSON(yamlBytes []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(yamlBytes, &node); err != nil {
		return nil, err
	}
	return encodeNodeToJSON(&node)
}

// ConvertJSONToYAML converts raw JSON bytes to YAML bytes while preserving exact key & value sequence.
func ConvertJSONToYAML(jsonBytes []byte) ([]byte, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(jsonBytes))
	if err := decoder.Decode(&node); err != nil {
		return nil, err
	}
	stripFlowStyle(&node)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		return nil, err
	}
	_ = encoder.Close()
	return buf.Bytes(), nil
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

func (cm *ConfigManager) SaveRawJSON(path string, jsonBytes []byte) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if path == "" {
		path = cm.configPath
	}

	yamlBytes, err := ConvertJSONToYAML(jsonBytes)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	if err := os.WriteFile(path, yamlBytes, 0644); err != nil {
		return err
	}

	var cfg model.CourseConfig
	if err := yaml.Unmarshal(yamlBytes, &cfg); err == nil {
		cm.active = cfg
	}

	cm.configPath = path
	cm.rawYAML = yamlBytes
	return nil
}

func (cm *ConfigManager) GenerateSkeletonYAML() ([]byte, error) {
	if cm.dal == nil {
		return nil, fmt.Errorf("DAL instance is nil")
	}
	subjects, err := cm.dal.FetchSubjects()
	if err != nil {
		return nil, err
	}
	modules, err := cm.dal.FetchModules()
	if err != nil {
		return nil, err
	}
	exercises, err := cm.dal.FetchExercises()
	if err != nil {
		return nil, err
	}

	modMap := make(map[int][]model.ExerciseEntity)
	for _, e := range exercises {
		modMap[e.ParentModuleID] = append(modMap[e.ParentModuleID], e)
	}

	subMap := make(map[int][]model.ModuleEntity)
	for _, m := range modules {
		subMap[m.ParentSubjectID] = append(subMap[m.ParentSubjectID], m)
	}

	var subjectsContent []*yaml.Node

	for _, s := range subjects {
		var modulesContent []*yaml.Node
		for _, m := range subMap[s.ID] {
			var exercisesContent []*yaml.Node
			for _, e := range modMap[m.ID] {
				exercisesContent = append(exercisesContent,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.Name},
					&yaml.Node{
						Kind: yaml.MappingNode,
						Tag:  "!!map",
						Content: []*yaml.Node{
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.Name},
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "score"},
							{Kind: yaml.ScalarNode, Tag: "!!float", Value: "100.0"},
						},
					},
				)
			}

			modulesContent = append(modulesContent,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: m.Name},
				&yaml.Node{
					Kind: yaml.MappingNode,
					Tag:  "!!map",
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: m.Name},
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "weight"},
						{Kind: yaml.ScalarNode, Tag: "!!float", Value: "1.0"},
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "exercises"},
						{Kind: yaml.MappingNode, Tag: "!!map", Content: exercisesContent},
					},
				},
			)
		}

		subjectsContent = append(subjectsContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s.Name},
			&yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: s.Name},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "weight"},
					{Kind: yaml.ScalarNode, Tag: "!!float", Value: "1.0"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "modules"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: modulesContent},
				},
			},
		)
	}

	docNode := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "subjects"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: subjectsContent},
				},
			},
		},
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(docNode); err != nil {
		return nil, err
	}
	_ = encoder.Close()
	return buf.Bytes(), nil
}

func (cm *ConfigManager) GenerateSkeleton() (model.CourseConfig, error) {
	yamlBytes, err := cm.GenerateSkeletonYAML()
	if err != nil {
		return model.CourseConfig{}, err
	}

	var cfg model.CourseConfig
	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return model.CourseConfig{}, err
	}
	return cfg, nil
}

func (cm *ConfigManager) GenerateSkeletonRawJSON() ([]byte, error) {
	yamlBytes, err := cm.GenerateSkeletonYAML()
	if err != nil {
		return nil, err
	}
	return ConvertYAMLToJSON(yamlBytes)
}

func (cm *ConfigManager) GenerateAndSaveRaw(path string) ([]byte, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	yamlBytes, err := cm.GenerateSkeletonYAML()
	if err != nil {
		return nil, err
	}

	if path == "" {
		path = cm.configPath
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	if err := os.WriteFile(path, yamlBytes, 0644); err != nil {
		return nil, err
	}

	var cfg model.CourseConfig
	if err := yaml.Unmarshal(yamlBytes, &cfg); err == nil {
		cm.active = cfg
	}

	cm.configPath = path
	cm.rawYAML = yamlBytes
	return ConvertYAMLToJSON(yamlBytes)
}

func (cm *ConfigManager) GenerateAndSave(path string) (model.CourseConfig, error) {
	_, err := cm.GenerateAndSaveRaw(path)
	return cm.GetActiveConfig(), err
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
	data, err := yaml.Marshal(cm.active)
	if err == nil {
		cm.rawYAML = data
		_ = os.WriteFile(cm.configPath, cm.rawYAML, 0644)
	}
	return cm.active, nil
}

func (cm *ConfigManager) PatchRawJSON(bodyBytes []byte) ([]byte, error) {
	var patchData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &patchData); err != nil {
		return nil, err
	}
	_, err := cm.Patch(patchData)
	if err != nil {
		return nil, err
	}
	return cm.GetActiveConfigRawJSON()
}
