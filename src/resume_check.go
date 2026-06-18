package main

import (
	"fmt"
	"log"
)

// checkResumeMarker looks for a resume.json left by upgrade-resume.
// If found, sends a system message and clears the marker.
func checkResumeMarker(agent *Agent) {
	m, err := loadResumeMarker()
	if err != nil {
		return
	}
	clearResumeMarker()
	msg := fmt.Sprintf("✅ Upgrade successful — resumed at commit %s", m.Version)
	agent.send(m.ChatID, msg)
	log.Printf("resume: sent resume message to chat %d for version %s", m.ChatID, m.Version)
}

