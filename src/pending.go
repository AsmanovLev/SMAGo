package main

import (
	"fmt"
	"strings"
	"sync"
)

type pendingMsg struct {
	Text      string
	MessageID int64
}

type pendingQueue struct {
	mu   sync.Mutex
	msgs map[int64][]pendingMsg
}

func newPendingQueue() *pendingQueue {
	return &pendingQueue{msgs: make(map[int64][]pendingMsg)}
}

func (pq *pendingQueue) Add(chatID int64, text string, messageID int64) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.msgs[chatID] = append(pq.msgs[chatID], pendingMsg{Text: text, MessageID: messageID})
}

func (pq *pendingQueue) Drain(chatID int64) []pendingMsg {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	msgs := pq.msgs[chatID]
	pq.msgs[chatID] = nil
	return msgs
}

func (pq *pendingQueue) Cancel(chatID int64) (pendingMsg, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	queue := pq.msgs[chatID]
	if len(queue) == 0 {
		return pendingMsg{}, false
	}
	msg := queue[len(queue)-1]
	pq.msgs[chatID] = queue[:len(queue)-1]
	return msg, true
}

func (pq *pendingQueue) Len(chatID int64) int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.msgs[chatID])
}

// sendQueued sends a message with a cancel button when agent is busy.
func (a *Agent) sendQueued(chatID int64, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	n := a.pending.Len(chatID) + 1
	botMsg := fmt.Sprintf("⬅ %s\n\nqueued for next step (%d pending)", truncateLog(text, 200), n)
	rows := [][]InlineButton{{{Text: "❌ Cancel", CallbackData: "pending-cancel"}}}
	msgID := a.tg.SendButtonsWithID(chatID, botMsg, rows)
	a.pending.Add(chatID, text, msgID)
}

// drainPending returns queued messages as a system message string for injection
// into the current Handle loop, and edits the queued messages to show "injected".
func (a *Agent) drainPending(chatID int64) string {
	msgs := a.pending.Drain(chatID)
	if len(msgs) == 0 {
		return ""
	}
	var combined string
	for _, m := range msgs {
		if combined != "" {
			combined += "\n"
		}
		combined += fmt.Sprintf("[user message]: %s", m.Text)
		_ = a.tg.EditMessageText(chatID, m.MessageID, fmt.Sprintf("✅ injected (%d messages)", len(msgs)), nil)
	}
	return combined
}

// handlePendingCancel handles the "pending-cancel" callback query.
func (a *Agent) handlePendingCancel(chatID int64, callbackID string) {
	msg, ok := a.pending.Cancel(chatID)
	if !ok {
		_ = a.tg.AnswerCallback(callbackID, "nothing to cancel")
		return
	}
	_ = a.tg.AnswerCallback(callbackID, "cancelled")
	_ = a.tg.EditMessageText(chatID, msg.MessageID, "❌ cancelled: "+msg.Text, nil)
}

// refreshSessionList edits an existing session list message to show the new active session.
func (a *Agent) refreshSessionList(chatID int64, msgID int64) {
	sessions, err := a.store.ListSessions(chatID)
	if err != nil {
		a.send(chatID, "❌ "+err.Error())
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📂 %d session(s):\n\n", len(sessions))
	for _, s := range sessions {
		marker := "  "
		if s.Active {
			marker = "✅"
		}
		age := humanAge(s.UpdatedAt)
		fmt.Fprintf(&b, "%s %s — %d msgs, %s\n", marker, s.Name, s.Messages, age)
	}
	b.WriteString("\ntap a session to switch:")
	var rows [][]InlineButton
	for _, s := range sessions {
		label := s.Name
		if s.Active {
			label += " ✅"
		}
		if s.Messages > 0 {
			label += fmt.Sprintf(" (%d)", s.Messages)
		}
		if len(label) > 40 {
			label = label[:40] + "…"
		}
		cb := "switch:" + s.Name
		if s.Active {
			cb = "noop"
		}
		rows = append(rows, []InlineButton{{Text: label, CallbackData: cb}})
	}
	_ = a.tg.EditMessageText(chatID, msgID, b.String(), rows)
}
