package main

import (
	"fmt"
	"log"
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

	// Restore session context
	sess, err := agent.store.LoadOrCreate(m.ChatID, "default")
	if err == nil {
		_ = sess.Append(ChatMessage{Role: "system",
			Content: fmt.Sprintf("Upgrade to %s successful. Continue your previous task.", m.Version)})
	}

	msg := fmt.Sprintf("✅ Upgrade successful — resumed at commit %s\nContinuing previous task…", m.Version)
	agent.send(m.ChatID, msg)
	log.Printf("resume: sent resume message to chat %d for version %s", m.ChatID, m.Version)

	// Push a continuation prompt so the LLM picks up where it left off
	_ = agent.Push(m.ChatID, "[system] Upgrade completed. Resume your previous task.")
}
