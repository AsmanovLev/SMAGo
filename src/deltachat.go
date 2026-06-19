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
	cfg        DeltaChatConfig
	rpc        *deltachat.Rpc
	accId      deltachat.AccountId
	dataDir    string
	running    bool
	cancel     context.CancelFunc
	inviteLink string
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
	chatmailRelays := map[string]bool{
		"chatmail.email": true, "nine.testrun.org": true, "mehl.cloud": true,
		"chatmail.woodpeckersnest.space": true, "chat.adminforge.de": true,
		"tarpit.fun": true, "chtml.ca": true, "danneskjold.de": true, "chat.vim.wtf": true,
	}
	if parts := strings.SplitN(d.cfg.Email, "@", 2); len(parts) == 2 {
		if chatmailRelays[parts[1]] {
			configs["configured_mail_server"] = parts[1]
			configs["configured_mail_port"] = "993"
			configs["configured_mail_user"] = d.cfg.Email
			configs["configured_mail_pw"] = d.cfg.Password
			configs["configured_send_server"] = parts[1]
			configs["configured_send_port"] = "465"
			configs["configured_send_user"] = d.cfg.Email
			configs["configured_send_pw"] = d.cfg.Password
		}
	}
	for k, v := range configs {
		d.rpc.SetConfig(d.accId, k, option.Some(v))
	}

	// Configure + StartIo
	for attempt := 0; attempt < 3; attempt++ {
		if err := d.rpc.Configure(d.accId); err != nil {
			log.Printf("deltachat: configure attempt %d: %v", attempt+1, err)
			time.Sleep(3 * time.Second)
			continue
		}
		if err := d.rpc.StartIo(d.accId); err != nil {
			log.Printf("deltachat: start io attempt %d: %v", attempt+1, err)
			time.Sleep(3 * time.Second)
			continue
		}
		d.running = true
		log.Printf("deltachat: IO started (attempt %d)", attempt+1)
		break
	}

	if !d.running {
		d.running = true
		log.Printf("deltachat: started (configure/startio failed, will retry)")
	}

	// Generate invite link (no bot.Run running, so RPC works)
	link, _, qrErr := d.rpc.GetChatSecurejoinQrCodeSvg(d.accId, option.None[deltachat.ChatId]())
	if qrErr != nil {
		log.Printf("deltachat: QR error: %v", qrErr)
	} else {
		d.inviteLink = link
		log.Printf("deltachat: invite link: %s", link)
	}

	// Start event loop (our own, using GetNextEvent)
	go d.eventLoop(rpcCtx)

	return nil
}

func (d *DeltaChatBackend) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		accId, event, err := d.rpc.GetNextEvent()
		if err != nil {
			log.Printf("deltachat: GetNextEvent error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		_ = accId
		switch e := event.(type) {
		case deltachat.EventIncomingMsg:
			d.handleIncoming(e)
		case deltachat.EventConnectivityChanged:
			// could log connectivity
		case deltachat.EventWarning:
			log.Printf("deltachat: warning")
		default:
			_ = e
		}
	}
}

func (d *DeltaChatBackend) handleIncoming(e deltachat.EventIncomingMsg) {
	msg, err := d.rpc.GetMessage(d.accId, e.MsgId)
	if err != nil || msg == nil || msg.ViewType != deltachat.MsgText {
		return
	}
	from := ""
	contacts, _ := d.rpc.GetChatContacts(d.accId, e.ChatId)
	if len(contacts) > 0 {
		cs, _ := d.rpc.GetContact(d.accId, contacts[0])
		if cs != nil {
			from = cs.Address
		}
	}
	log.Printf("deltachat: from=%s chat=%d text=%s", from, e.ChatId, truncateLog(msg.Text, 100))
	d.injectToAgent(msg.Text, e.ChatId)
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

func (d *DeltaChatBackend) GetInviteLink() string {
	return d.inviteLink
}

func (d *DeltaChatBackend) IsRunning() bool { return d.running }

func (d *DeltaChatBackend) Stop() {
	if d.cancel != nil {
		d.cancel()
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
