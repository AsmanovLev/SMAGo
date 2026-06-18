package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// progressReader wraps an io.Reader and reports progress periodically (every ~5s).
type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	fn       func(downloaded, total int64, done bool, err error)
	lastTime time.Time
	minDelta int64
}

func newProgressReader(r io.Reader, total int64, fn func(downloaded, total int64, done bool, err error)) *progressReader {
	// Min 1% or 256KB, whichever is larger
	minDelta := total / 100
	if minDelta < 256*1024 {
		minDelta = 256 * 1024
	}
	return &progressReader{r: r, total: total, fn: fn, lastTime: time.Now().Add(-10 * time.Second), minDelta: minDelta}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read += int64(n)
	if pr.fn != nil && pr.total > 0 && time.Since(pr.lastTime) >= 5*time.Second {
		// Throttle: at most once per 5 seconds
		if pr.read >= pr.minDelta {
			pr.lastTime = time.Now()
			pr.fn(pr.read, pr.total, false, nil)
		}
	}
	return n, err
}

type Telegram struct {
	token  string
	client *http.Client
	offset int64
	proxy  string
}

type TGUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		From      *struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text    string `json:"text"`
		Caption string `json:"caption"`
		Document *struct {
			FileID   string `json:"file_id"`
			FileName string `json:"file_name"`
			FileSize int64  `json:"file_size"`
			MimeType string `json:"mime_type"`
		} `json:"document"`
		Photo []struct {
			FileID   string `json:"file_id"`
			FileSize int64  `json:"file_size"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
		} `json:"photo"`
	} `json:"message"`
	CallbackQuery *struct {
		ID      string `json:"id"`
		From    *struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Message *struct {
			MessageID int64 `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
		Data string `json:"data"`
	} `json:"callback_query"`
}

type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type TGResponse struct {
	OK     bool        `json:"ok"`
	Result []TGUpdate  `json:"result"`
	Error  interface{} `json:"error,omitempty"`
}

type TGMe struct {
	OK     bool `json:"ok"`
	Result *struct {
		ID        int64  `json:"id"`
		IsBot     bool   `json:"is_bot"`
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
	} `json:"result"`
}

func NewTelegram(token string) *Telegram {
	return &Telegram{token: token, client: &http.Client{Timeout: 35 * time.Second}}
}

func (t *Telegram) SetProxyURL(rawURL string) error {
	if rawURL == "" {
		t.client.Transport = nil
		t.proxy = ""
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	t.client.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
	t.proxy = rawURL
	return nil
}

func (t *Telegram) GetMe(ctx context.Context) (*TGMe, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.telegram.org/bot"+t.token+"/getMe", nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m TGMe
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	if !m.OK || m.Result == nil {
		return nil, fmt.Errorf("telegram: getMe returned ok=false")
	}
	return &m, nil
}

func (t *Telegram) LongPoll(ctx context.Context) (*TGUpdate, error) {
	for ctx.Err() == nil {
		v := url.Values{}
		v.Set("timeout", "30")
		v.Set("offset", fmt.Sprintf("%d", t.offset))
		v.Set("allowed_updates", `["message","callback_query"]`)
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.telegram.org/bot"+t.token+"/getUpdates?"+v.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := t.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			time.Sleep(2 * time.Second)
			continue
		}
		var tr TGResponse
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Body.Close()
		if !tr.OK {
			time.Sleep(2 * time.Second)
			continue
		}
		if len(tr.Result) > 0 {
			t.offset = tr.Result[len(tr.Result)-1].UpdateID + 1
			for _, u := range tr.Result {
				if u.Message != nil && u.Message.Text != "" {
					return &u, nil
				}
				if u.CallbackQuery != nil {
					return &u, nil
				}
			}
		}
	}
	return nil, ctx.Err()
}

func (t *Telegram) SendChatAction(chatID int64, action string) error {
	v := url.Values{}
	v.Set("chat_id", fmt.Sprintf("%d", chatID))
	v.Set("action", action)
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/sendChatAction", strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (t *Telegram) Typing(chatID int64) error {
	return t.SendChatAction(chatID, "typing")
}

func (t *Telegram) Send(chatID int64, text string) error {
	return t.SendButtons(chatID, text, nil)
}

func (t *Telegram) SendPlain(chatID int64, text string) error {
	return t.sendMessage(chatID, text, false)
}

func (t *Telegram) SendSilent(chatID int64, text string) error {
	return t.sendMessage(chatID, text, true)
}

func (t *Telegram) sendMessage(chatID int64, text string, silent bool) error {
	if len(text) > 4000 {
		text = text[:4000] + "\n\n[...truncated]"
	}
	v := url.Values{}
	v.Set("chat_id", fmt.Sprintf("%d", chatID))
	v.Set("text", mdToTelegramHTML(text))
	v.Set("parse_mode", "HTML")
	if silent {
		v.Set("disable_notification", "true")
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/sendMessage", strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (t *Telegram) SendButtons(chatID int64, text string, rows [][]InlineButton) error {
	if len(text) > 4000 {
		text = text[:4000] + "\n\n[...truncated]"
	}
	v := url.Values{}
	v.Set("chat_id", fmt.Sprintf("%d", chatID))
	v.Set("text", mdToTelegramHTML(text))
	v.Set("parse_mode", "HTML")
	if len(rows) > 0 {
		kb, _ := json.Marshal(map[string]any{"inline_keyboard": rows})
		v.Set("reply_markup", string(kb))
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/sendMessage", strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}


// SendButtonsWithID is like SendButtons but returns the sent message ID.
func (t *Telegram) SendButtonsWithID(chatID int64, text string, rows [][]InlineButton) int64 {
	if len(text) > 4000 {
		text = text[:4000] + "\n\n[...truncated]"
	}
	v := url.Values{}
	v.Set("chat_id", fmt.Sprintf("%d", chatID))
	v.Set("text", mdToTelegramHTML(text))
	v.Set("parse_mode", "HTML")
	if len(rows) > 0 {
		kb, _ := json.Marshal(map[string]any{"inline_keyboard": rows})
		v.Set("reply_markup", string(kb))
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/sendMessage", strings.NewReader(v.Encode()))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.OK {
		return result.Result.MessageID
	}
	return 0
}


func (t *Telegram) AnswerCallback(callbackID, text string) error {
	v := url.Values{}
	v.Set("callback_query_id", callbackID)
	if text != "" {
		v.Set("text", text)
		v.Set("show_alert", "false")
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/answerCallbackQuery", strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func (t *Telegram) SetMyCommands(commands []BotCommand) error {
	payload, err := json.Marshal(map[string]any{"commands": commands, "scope": map[string]any{"type": "default"}})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/setMyCommands", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("setMyCommands HTTP %d", resp.StatusCode)
	}
	return nil
}

func (t *Telegram) EditMessageText(chatID int64, messageID int64, text string, rows [][]InlineButton) error {
	if len(text) > 4000 {
		text = text[:4000] + "\n\n[...truncated]"
	}
	v := url.Values{}
	v.Set("chat_id", fmt.Sprintf("%d", chatID))
	v.Set("message_id", fmt.Sprintf("%d", messageID))
	v.Set("text", mdToTelegramHTML(text))
	v.Set("parse_mode", "HTML")
	if len(rows) > 0 {
		kb, _ := json.Marshal(map[string]any{"inline_keyboard": rows})
		v.Set("reply_markup", string(kb))
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/editMessageText", strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// SetReplyKeyboard shows a persistent reply keyboard (bottom bar) for a chat.
// It sends a "." dot message with the keyboard attached, then deletes it,
// leaving just the keyboard visible.
func (t *Telegram) SetReplyKeyboard(chatID int64, buttons [][]string) error {
	kb := map[string]any{
		"keyboard":          buttons,
		"resize_keyboard":   true,
		"one_time_keyboard": false,
	}
	if buttons == nil {
		kb = map[string]any{
			"keyboard":        [][]string{},
			"resize_keyboard": true,
			"remove_keyboard": true,
		}
	}
	kbJSON, _ := json.Marshal(kb)

	// Send a dot message with the keyboard
	v := url.Values{}
	v.Set("chat_id", fmt.Sprintf("%d", chatID))
	v.Set("text", ".")
	v.Set("reply_markup", string(kbJSON))
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/sendMessage", strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	// Delete the dot message so only the keyboard remains
	if result.OK && result.Result.MessageID != 0 {
		dv := url.Values{}
		dv.Set("chat_id", fmt.Sprintf("%d", chatID))
		dv.Set("message_id", fmt.Sprintf("%d", result.Result.MessageID))
		dreq, _ := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/deleteMessage", strings.NewReader(dv.Encode()))
		if dreq != nil {
			dreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			dresp, derr := t.client.Do(dreq)
			if derr == nil {
				dresp.Body.Close()
			}
		}
	}
	return nil
}

const MaxFileSize = 50 * 1024 * 1024 // 50 MB

// DownloadFile downloads a Telegram file by file_id to destPath.
// progress is called periodically with (downloaded, total, done, err).
func (t *Telegram) DownloadFile(fileID string, destPath string, progress func(downloaded, total int64, done bool, err error)) error {
	v := url.Values{}
	v.Set("file_id", fileID)
	req, err := http.NewRequest("GET", "https://api.telegram.org/bot"+t.token+"/getFile?"+v.Encode(), nil)
	if err != nil {
		if progress != nil { progress(0, 0, true, err) }
		return err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		if progress != nil { progress(0, 0, true, err) }
		return err
	}
	defer resp.Body.Close()
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
			FileSize int64  `json:"file_size"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if progress != nil { progress(0, 0, true, err) }
		return err
	}
	if !result.OK {
		err := fmt.Errorf("getFile failed")
		if progress != nil { progress(0, 0, true, err) }
		return err
	}
	total := result.Result.FileSize
	if total > MaxFileSize {
		err := fmt.Errorf("file too large: %d bytes (limit %d MB)", total, MaxFileSize/(1024*1024))
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	fileURL := "https://api.telegram.org/file/bot" + t.token + "/" + result.Result.FilePath
	dlReq, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	dlResp, err := t.client.Do(dlReq)
	if err != nil {
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode >= 400 {
		err := fmt.Errorf("download HTTP %d", dlResp.StatusCode)
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	defer f.Close()
	// Wrap body with progress tracking
	var downloaded int64
	pr := newProgressReader(dlResp.Body, total, progress)
	n, err := io.Copy(f, pr)
	if err != nil {
		if progress != nil { progress(downloaded, total, true, err) }
		return err
	}
	if n > MaxFileSize {
		os.Remove(destPath)
		err := fmt.Errorf("file too large: %d bytes", n)
		if progress != nil { progress(n, total, true, err) }
		return err
	}
	if progress != nil { progress(total, total, true, nil) }
	return nil
}

// SendDocument sends a file to a Telegram chat.
// progress is called periodically with (sent, total, done, err).
func (t *Telegram) SendDocument(chatID int64, filePath string, caption string, progress func(sent, total int64, done bool, err error)) error {
	f, err := os.Open(filePath)
	if err != nil {
		if progress != nil { progress(0, 0, true, err) }
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		if progress != nil { progress(0, 0, true, err) }
		return err
	}
	total := info.Size()
	if total > MaxFileSize {
		err := fmt.Errorf("file too large: %d bytes (limit %d MB)", total, MaxFileSize/(1024*1024))
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	// Build multipart body with progress tracking
	var body bytes.Buffer
	boundary := "----SMAGoBoundary"
	fmt.Fprintf(&body, "--%s\r\n", boundary)
	fmt.Fprintf(&body, "Content-Disposition: form-data; name=\"chat_id\"\r\n\r\n")
	fmt.Fprintf(&body, "%d\r\n", chatID)
	if caption != "" {
		fmt.Fprintf(&body, "--%s\r\n", boundary)
		fmt.Fprintf(&body, "Content-Disposition: form-data; name=\"caption\"\r\n\r\n")
		fmt.Fprintf(&body, "%s\r\n", caption)
	}
	fileName := filepath.Base(filePath)
	fmt.Fprintf(&body, "--%s\r\n", boundary)
	fmt.Fprintf(&body, "Content-Disposition: form-data; name=\"document\"; filename=\"%s\"\r\n", fileName)
	fmt.Fprintf(&body, "Content-Type: application/octet-stream\r\n\r\n")
	io.Copy(&body, f)
	fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)
	if progress != nil {
		progress(0, total, false, nil) // starting upload
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+t.token+"/sendDocument", bytes.NewReader(body.Bytes()))
	if err != nil {
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.ContentLength = int64(body.Len())
	resp, err := t.client.Do(req)
	if err != nil {
		if progress != nil { progress(0, total, true, err) }
		return err
	}
	defer resp.Body.Close()
	if progress != nil { progress(total, total, true, nil) }
	return nil
}
