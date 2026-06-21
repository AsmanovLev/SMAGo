package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deltachat/deltachat-rpc-client-go/deltachat"
	"github.com/deltachat/deltachat-rpc-client-go/deltachat/option"
	"github.com/deltachat/deltachat-rpc-client-go/deltachat/transport"
)

const lang = "go2"

func main() {
	exe, err := os.Executable()
	must(err, "executable")
	root := filepath.Dir(filepath.Dir(filepath.Dir(exe))) // go2/ -> go/ -> deltachat-tests/

	cfgRaw, err := os.ReadFile(filepath.Join(root, "common.json"))
	must(err, "read common.json")
	var cfg struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	must(json.Unmarshal(cfgRaw, &cfg), "parse common.json")

	inviteBytes, err := os.ReadFile(filepath.Join(root, "invite.txt"))
	must(err, "read invite.txt")
	invite := strings.TrimSpace(string(inviteBytes))

	dbDir := filepath.Join(root, "_runtime", "deltachat-db")
	must(os.MkdirAll(dbDir, 0o755), "mkdir db")

	t := transport.NewIOTransport()
	t.AccountsDir = dbDir
	must(t.Open(), "transport open")
	defer t.Close()

	ctx := context.Background()
	rpc := &deltachat.Rpc{Context: ctx, Transport: t}

	accIds, _ := rpc.GetAllAccountIds()
	var accId deltachat.AccountId
	if len(accIds) == 0 {
		accId, err = rpc.AddAccount()
		must(err, "add_account")
	} else {
		accId = accIds[0]
	}
	fmt.Printf("[%s] account=%d\n", lang, accId)

	for k, v := range map[string]string{
		"addr":                   cfg.Email,
		"mail_pw":                cfg.Password,
		"displayname":            cfg.Name,
		"configured_mail_server": "imap.rambler.ru",
		"configured_mail_port":   "993",
		"configured_mail_user":   cfg.Email,
		"configured_mail_pw":     cfg.Password,
		"configured_send_server": "smtp.rambler.ru",
		"configured_send_port":   "465",
		"configured_send_user":   cfg.Email,
		"configured_send_pw":     cfg.Password,
	} {
		err := rpc.SetConfig(accId, k, option.Some(v))
		must(err, fmt.Sprintf("set_config %s", k))
	}

	imapDone := make(chan struct{})
	bot := deltachat.NewBot(rpc)
	bot.On(deltachat.EventImapConnected{}, func(b *deltachat.Bot, a deltachat.AccountId, e deltachat.Event) {
		ec := e.(deltachat.EventImapConnected)
		fmt.Printf("[%s] imap: %s\n", lang, ec.Msg)
		if a == accId {
			select {
			case <-imapDone:
			default:
				close(imapDone)
			}
		}
	})
	bot.On(deltachat.EventSmtpConnected{}, func(b *deltachat.Bot, a deltachat.AccountId, e deltachat.Event) {
		fmt.Printf("[%s] smtp: %s\n", lang, e.(deltachat.EventSmtpConnected).Msg)
	})
	bot.On(deltachat.EventInfo{}, func(b *deltachat.Bot, a deltachat.AccountId, e deltachat.Event) {
		if a == accId {
			fmt.Printf("[%s] info: %s\n", lang, e.(deltachat.EventInfo).Msg)
		}
	})

	must(rpc.Configure(accId), "configure")
	must(rpc.StartIo(accId), "start_io")
	go bot.Run()

	select {
	case <-imapDone:
	case <-time.After(90 * time.Second):
		die("imap never connected")
	}

	chatId, err := rpc.SecureJoin(accId, invite)
	must(err, "securejoin")
	fmt.Printf("[%s] secure-join chat=%d\n", lang, chatId)

	joinerReady := make(chan struct{})
	bot.On(deltachat.EventSecurejoinJoinerProgress{}, func(b *deltachat.Bot, a deltachat.AccountId, e deltachat.Event) {
		ej := e.(deltachat.EventSecurejoinJoinerProgress)
		fmt.Printf("[%s] secure-join progress: contact=%d progress=%d\n", lang, ej.ContactId, ej.Progress)
		if a == accId && ej.Progress >= 400 {
			select {
			case <-joinerReady:
			default:
				close(joinerReady)
			}
		}
	})

	select {
	case <-joinerReady:
	case <-time.After(60 * time.Second):
		die("secure-join timeout")
	}
	time.Sleep(2 * time.Second)

	ts := time.Now().UTC().Format(time.RFC3339)
	text := fmt.Sprintf("yo, new chat. ping #%d from %s @ %s", chatId, lang, ts)
	var msgId deltachat.MsgId
	for attempt := 1; attempt <= 5; attempt++ {
		msgId, err = rpc.SendMsg(accId, chatId, deltachat.MsgData{Text: text})
		if err == nil {
			break
		}
		fmt.Printf("[%s] send attempt %d failed: %v\n", lang, attempt, err)
		if attempt == 5 {
			die("send_msg: " + err.Error())
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Printf("[%s] sent msg=%d text=%q\n", lang, msgId, text)
	bot.Stop()
	_ = rpc.StopIo(accId)
	fmt.Printf("[%s] OK\n", lang)
}

func must(err error, what string) {
	if err != nil {
		die(what + ": " + err.Error())
	}
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "[go2] FAIL: %s\n", msg)
	os.Exit(1)
}
