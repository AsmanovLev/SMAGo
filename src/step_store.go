package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type StepStore struct {
	path string
}

type StepData struct {
	Steps map[int64]int
}

func NewStepStore(dataDir string) *StepStore {
	return &StepStore{path: filepath.Join(dataDir, "steps.json")}
}

func (s *StepStore) Get(chatID int64) int {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return 0
	}
	var d StepData
	if err := json.Unmarshal(data, &d); err != nil {
		return 0
	}
	return d.Steps[chatID]
}

func (s *StepStore) Set(chatID int64, step int) {
	d := StepData{Steps: make(map[int64]int)}
	if data, err := os.ReadFile(s.path); err == nil {
		json.Unmarshal(data, &d)
	}
	if d.Steps == nil {
		d.Steps = make(map[int64]int)
	}
	d.Steps[chatID] = step
	data, _ := json.Marshal(d)
	os.WriteFile(s.path, data, 0644)
}
