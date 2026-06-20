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


// refreshModelGrid edits an existing model grid message to show the new active model.
func (a *Agent) refreshModelGrid(chatID int64, msgID int64) {
	prov, ok := a.cfg.Providers[a.cfg.Provider]
	if !ok {
		return
	}
	var rows [][]InlineButton
	for name, m := range prov.Models {
		label := "• " + name
		if name == a.cfg.DefaultModel {
			label += " ✅"
		}
		if m.ContextWindow > 0 {
			label += fmt.Sprintf("  (%dk ctx)", m.ContextWindow/1000)
		}
		if len(label) > 60 {
			label = label[:60] + "…"
		}
		rows = append(rows, []InlineButton{{Text: label, CallbackData: "model:" + name}})
	}
	cw := a.getModelContextWindow()
	dcpStatus := "OFF"
	if a.cfg.DCP.Enabled {
		dcpStatus = "ON"
	}
	info := fmt.Sprintf("🤖 provider: %s\npick a model:\n\n📦 DCP: %s", a.cfg.Provider, dcpStatus)
	if cw > 0 {
		info += fmt.Sprintf("\ncontext: %dk | min 20%%: %dk | max 80%%: %dk", cw/1000, a.cfg.DCP.MinContextTokens/1000, a.cfg.DCP.MaxContextTokens/1000)
	}
	_ = a.tg.EditMessageText(chatID, msgID, info, rows)
}

// refreshProviderGrid edits an existing provider grid message to show the new active provider.
func (a *Agent) refreshProviderGrid(chatID int64, msgID int64) {
	if len(a.cfg.Providers) == 0 {
		return
	}
	var rows [][]InlineButton
	for name, p := range a.cfg.Providers {
		modelCount := len(p.Models)
		current := ""
		if name == a.cfg.Provider {
			current = " ✅"
		}
		label := fmt.Sprintf("• %s — %d model(s)%s", p.Name, modelCount, current)
		if len(label) > 60 {
			label = label[:60] + "…"
		}
		rows = append(rows, []InlineButton{{Text: label, CallbackData: "provider:" + name}})
	}
	_ = a.tg.EditMessageText(chatID, msgID, "🤖 pick a provider:", rows)
}

// refreshSessionList edits an existing session list message to show the new active session.
func (a *Agent) refreshSessionList(chatID int64, msgID int64) {
	sessions, err := a.store.ListSessions(chatID)
	if err != nil {
		a.send(chatID, "❌ "+err.Error())
		return
	}
	var rows [][]InlineButton
	for _, s := range sessions {
		icon := "⚪"
		if s.Active {
			icon = "🔘"
		}
		label := fmt.Sprintf("%s (%d)", s.Name, s.Messages)
		if len(label) > 36 {
			label = label[:36] + "…"
		}
		cb := "switch:" + s.Name
		if s.Active {
			cb = "noop"
		}
		rows = append(rows, []InlineButton{{Text: icon + " " + label, CallbackData: cb}})
	}
	msg := fmt.Sprintf("📂 %d session(s)", len(sessions))
	_ = a.tg.EditMessageText(chatID, msgID, msg, rows)
}
