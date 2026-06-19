package main

import (
	"context"
	"time"
)

// statusLoop repeats a Telegram chat action every 4 seconds until stopped.
type statusLoop struct {
	cancel func()
}

func (s *statusLoop) stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (a *Agent) startStatus(chatID int64, actionFn func(int64)) *statusLoop {
	ctx, cancel := context.WithCancel(context.Background())
	actionFn(chatID)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				actionFn(chatID)
			}
		}
	}()
	return &statusLoop{cancel: cancel}
}

// beginThinking shows "choose a sticker" while LLM processes.
func (a *Agent) beginThinking(chatID int64) *statusLoop {
	return a.startStatus(chatID, a.chooseSticker)
}

// beginToolCall shows "playing" while a tool executes.
func (a *Agent) beginToolCall(chatID int64) *statusLoop {
	return a.startStatus(chatID, a.playing)
}

// beginTyping shows "typing..." while text is being generated.
func (a *Agent) beginTyping(chatID int64) *statusLoop {
	return a.startStatus(chatID, a.typing)
}
