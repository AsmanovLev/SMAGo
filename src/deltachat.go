package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/deltachat/deltachat-rpc-client-go/deltachat"
	"github.com/deltachat/deltachat-rpc-client-go/deltachat/option"
	"github.com/deltachat/deltachat-rpc-client-go/deltachat/transport"
)

type DeltaChatConfig struct {
	Enabled  bool   `json:"enabled"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

var globalAgent *Agent

type DeltaChatBackend struct {
	cfg     DeltaChatConfig
	rpc     *deltachat.Rpc
	bot     *deltachat.Bot
	accId   deltachat.AccountId
	dataDir string
	running bool
	cancel  context.CancelFunc
}

func NewDeltaChatBackend(cfg DeltaChatConfig, dataDir string) *DeltaChatBackend {
	if !cfg.Enabled || cfg.Email == "" {
		return nil
	}
	return &DeltaChatBackend{cfg: cfg, dataDir: dataDir}
}

func (d *DeltaChatBackend) Start(ctx context.Context) error {
	rpcDir := filepath.Join(d.dataDir, "deltachat-db")
	os.MkdirAll(rpcDir, 0755)

	rpcServer, err := findDeltachatRPCServer()
	if err != nil {
		return fmt.Errorf("deltachat-rpc-server: %w", err)
	}
	log.Printf("deltachat: rpc server at %s", rpcServer)

	t := transport.NewIOTransport()
	t.AccountsDir = rpcDir
	if err := t.Open(); err != nil {
		return fmt.Errorf("rpc transport: %w", err)
	}

	rpcCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	d.rpc = &deltachat.Rpc{Context: rpcCtx, Transport: t}
	d.bot = deltachat.NewBot(d.rpc)

	accIds, _ := d.rpc.GetAllAccountIds()
	if len(accIds) == 0 {
		d.accId, _ = d.rpc.AddAccount()
		log.Printf("deltachat: created account %d", d.accId)
	} else {
		d.accId = accIds[0]
		log.Printf("deltachat: using account %d", d.accId)
	}

	configs := map[string]string{
		"addr":    d.cfg.Email,
		"mail_pw": d.cfg.Password,
	}
	if d.cfg.Name != "" {
		configs["display_name"] = d.cfg.Name
	}
	// Chatmail relays: no imap./smtp. subdomains, force via configured_* keys
	// Standard providers (Rambler, Gmail, etc.) are auto-detected by DC core
	chatmailRelays := map[string]bool{
		"chatmail.email": true,
		"nine.testrun.org": true,
		"mehl.cloud": true,
		"chatmail.woodpeckersnest.space": true,
		"chat.adminforge.de": true,
		"tarpit.fun": true,
		"chtml.ca": true,
		"danneskjold.de": true,
		"chat.vim.wtf": true,
	}
	if parts := strings.SplitN(d.cfg.Email, "@", 2); len(parts) == 2 {
		relay := parts[1]
		if chatmailRelays[relay] {
			configs["configured_mail_server"] = relay
			configs["configured_mail_port"] = "993"
			configs["configured_mail_user"] = d.cfg.Email
			configs["configured_mail_pw"] = d.cfg.Password
			configs["configured_send_server"] = relay
			configs["configured_send_port"] = "465"
			configs["configured_send_user"] = d.cfg.Email
			configs["configured_send_pw"] = d.cfg.Password
		}
	}
	for k, v := range configs {
		d.rpc.SetConfig(d.accId, k, option.Some(v))
	}

	if err := d.rpc.Configure(d.accId); err != nil {
		cancel()
		return fmt.Errorf("configure: %w", err)
	}
	if err := d.rpc.StartIo(d.accId); err != nil {
		cancel()
		return fmt.Errorf("start io: %w", err)
	}
	d.running = true
	log.Printf("deltachat: IO started")

	d.bot.OnNewMsg(d.handleNewMessage)
	go func() {
		d.bot.Run()
		d.running = false
	}()

	return nil
}

func (d *DeltaChatBackend) handleNewMessage(bot *deltachat.Bot, accId deltachat.AccountId, msgId deltachat.MsgId) {
	msg, err := d.rpc.GetMessage(accId, msgId)
	if err != nil || msg == nil || msg.FromId == 0 || msg.ViewType != deltachat.MsgText {
		return
	}

	from := ""
	contacts, _ := d.rpc.GetChatContacts(accId, msg.ChatId)
	if len(contacts) > 0 {
		cs, _ := d.rpc.GetContact(accId, contacts[0])
		if cs != nil {
			from = cs.Address
		}
	}

	log.Printf("deltachat: from=%s chat=%d text=%s", from, msg.ChatId, truncateLog(msg.Text, 100))
	d.injectToAgent(msg.Text, msg.ChatId)
}

func (d *DeltaChatBackend) injectToAgent(text string, chatId deltachat.ChatId) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		if globalAgent == nil {
			log.Printf("deltachat: agent not ready")
			return
		}
		reply, err := globalAgent.Handle(0, text)
		if err != nil {
			log.Printf("deltachat: handle error: %v", err)
			return
		}
		d.rpc.SendMsg(d.accId, chatId, deltachat.MsgData{Text: reply})
		log.Printf("deltachat: reply sent (%d chars)", len(reply))
	}()
}

func (d *DeltaChatBackend) Send(chatId deltachat.ChatId, text string) error {
	if d.rpc == nil {
		return fmt.Errorf("not started")
	}
	_, err := d.rpc.SendMsg(d.accId, chatId, deltachat.MsgData{Text: text})
	return err
}

func (d *DeltaChatBackend) StartChat(email, name string) (deltachat.ChatId, error) {
	if d.rpc == nil {
		return 0, fmt.Errorf("not started")
	}
	contactId, err := d.rpc.CreateContact(d.accId, email, name)
	if err != nil {
		return 0, fmt.Errorf("create contact: %w", err)
	}
	log.Printf("deltachat: contact created id=%d addr=%s", contactId, email)
	chatId, err := d.rpc.CreateChatByContactId(d.accId, contactId)
	if err != nil {
		return 0, fmt.Errorf("create chat: %w", err)
	}
	log.Printf("deltachat: chat created id=%d", chatId)
	return chatId, nil
}

func (d *DeltaChatBackend) GetQRCode(chatId deltachat.ChatId) (string, string, error) {
	if d.rpc == nil {
		return "", "", fmt.Errorf("not started")
	}
	return d.rpc.GetChatSecurejoinQrCodeSvg(d.accId, option.Some(chatId))
}

func (d *DeltaChatBackend) IsRunning() bool { return d.running }

func (d *DeltaChatBackend) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.bot != nil {
		d.bot.Stop()
	}
	if d.rpc != nil {
		d.rpc.StopIo(d.accId)
	}
	d.running = false
}

func findDeltachatRPCServer() (string, error) {
	paths := []string{"deltachat-rpc-server"}
	appData := os.Getenv("APPDATA")
	if appData != "" {
		paths = append(paths, filepath.Join(appData, "Python", "Scripts", "deltachat-rpc-server.exe"))
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("deltachat-rpc-server"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("deltachat-rpc-server not found")
}

func isDeltaChatMessage(headers string) bool {
	return strings.Contains(headers, "Chat-Version:")
}