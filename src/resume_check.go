package main

import (
	"fmt"
	"log"
	"time"
)

// checkResumeMarker looks for a resume.json left by upgrade-resume.
// If found, sends a system message, restores session context, and
// pushes a continuation prompt so the agent resumes work.
func checkResumeMarker(agent *Agent) {
	m, err := loadResumeMarker()
	if err != nil {
		return
	}
	clearResumeMarker()

	// Append to the ACTIVE session, not a hardcoded "default"
	sess, err := agent.store.GetActive(m.ChatID)
	if err != nil {
		sess, err = agent.store.LoadOrCreate(m.ChatID, "new-resume")
	}
	if err == nil {
		_ = sess.Append(ChatMessage{Role: "system",
			Content: fmt.Sprintf("Upgrade to %s successful. Continue your previous task.", m.Version)})
	}

	msg := fmt.Sprintf("Upgrade to %s successful. Continue your previous task.", m.Version)
	log.Printf("resume: sending resume to chat %d for version %s", m.ChatID, m.Version)

	// Delay the push so RunLoop has time to start reading from inject channel
	go func() {
		time.Sleep(2 * time.Second)
		if err := agent.Push(m.ChatID, msg); err != nil {
			log.Printf("resume: push failed: %v", err)
		}
	}()
}
