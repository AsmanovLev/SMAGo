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

func saveResumeMarker(chatID int64, version string, steps int) error {
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

