import os, sys, json, tempfile, shutil
from pathlib import Path

# Write the Go test file
go_code = """package main

import (
\t"context"
\t"fmt"
\t"os"
\t"path/filepath"
\t"time"

\t"github.com/deltachat/deltachat-rpc-client-go/deltachat"
\t"github.com/deltachat/deltachat-rpc-client-go/deltachat/option"
\t"github.com/deltachat/deltachat-rpc-client-go/deltachat/transport"
)

func main() {
\tpass := "2ZdXFM*yot7Hzz^t"

\trpcDir := filepath.Join(os.TempDir(), "smago-dc-invite")
\tos.RemoveAll(rpcDir)
\tos.MkdirAll(rpcDir, 0755)

\tt := transport.NewIOTransport()
\tt.AccountsDir = rpcDir
\tif err := t.Open(); err != nil {
\t\tfmt.Println("ERROR:", err)
\t\treturn
\t}
\tdefer t.Close()

\tctx := context.Background()
\trpc := &deltachat.Rpc{Context: ctx, Transport: t}
\tbot := deltachat.NewBot(rpc)

\taccIds, _ := rpc.GetAllAccountIds()
\tvar accId deltachat.AccountId
\tif len(accIds) == 0 {
\t\taccId, _ = rpc.AddAccount()
\t} else {
\t\taccId = accIds[0]
\t}

\trpc.SetConfig(accId, "addr", option.Some("sma.go@ro.ru"))
\trpc.SetConfig(accId, "mail_pw", option.Some(pass))
\trpc.SetConfig(accId, "display_name", option.Some("SMAGo"))

\tfmt.Println("Configuring...")
\tif err := rpc.Configure(accId); err != nil {
\t\tfmt.Println("ERROR:", err)
\t\treturn
\t}
\tfmt.Println("Configured!")
\trpc.StartIo(accId)
\tfmt.Println("IO started")

\t// Listen for events to log connectivity
\tbot.OnNewMsg(func(bot *deltachat.Bot, accId deltachat.AccountId, msgId deltachat.MsgId) {
\t\tfmt.Printf("NEW MSG: %d\\n", msgId)
\t})
\tbot.OnUnhandledEvent(func(bot *deltachat.Bot, accId deltachat.AccountId, event deltachat.Event) {
\t\tfmt.Printf("EVENT: %T\\n", event)
\t})
\tgo bot.Run()

\tfmt.Println("Waiting 20s for connectivity...")
\ttime.Sleep(20 * time.Second)

\t// Try getting QR code for secure join
\tfmt.Println("Getting QR code...")
\tsvg, qrdata, err := rpc.GetChatSecurejoinQrCodeSvg(accId, option.None[deltachat.ChatId]())
\tif err != nil {
\t\tfmt.Println("QR error:", err)
\t} else {
\t\tfmt.Printf("QR data: %s\\n", qrdata)
\t\t// Write SVG to file
\t\tpath := filepath.Join(os.TempDir(), "smago-dc-invite", "qr.svg")
\t\tos.WriteFile(path, []byte(svg), 0644)
\t\tfmt.Printf("QR SVG saved to: %s\\n", path)
\t}

\tbot.Stop()
}
"""

go_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "dc_test.go")
with open(go_path, "w", encoding="utf-8") as f:
    f.write(go_code)
print(f"Wrote {go_path}")

