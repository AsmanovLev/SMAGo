package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ResumeMarker struct {
	ChatID  int64  `json:"chat_id"`
	Version string `json:"version"`
	Steps   int    `json:"steps,omitempty"`
}

func resumePath() string {
	return filepath.Join(projectRoot(), "data", "resume.json")
}

func stepsPath() string {
	return filepath.Join(projectRoot(), "data", "steps.json")
}

func readStepFromStore(chatID int64) int {
	data, err := os.ReadFile(stepsPath())
	if err != nil {
		return 0
	}
	var d struct {
		Steps map[int64]int
	}
	if err := json.Unmarshal(data, &d); err != nil || d.Steps == nil {
		return 0
	}
	return d.Steps[chatID]
}

func saveResumeMarker(chatID int64, version string, steps int) error {
	// If caller passed 0, try reading from persistent step store
	if steps == 0 {
		steps = readStepFromStore(chatID)
	}
	m := ResumeMarker{ChatID: chatID, Version: version, Steps: steps}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(resumePath(), data, 0644)
}

func loadResumeMarker() (*ResumeMarker, error) {
	data, err := os.ReadFile(resumePath())
	if err != nil {
		return nil, err
	}
	var m ResumeMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func clearResumeMarker() {
	_ = os.Remove(resumePath())
}
