package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

const emailMaxFileSize = 50 * 1024 * 1024

type EmailConfig struct {
	Enabled  bool   `json:"enabled"`
	IMAPHost string `json:"imapHost"`
	IMAPPort int    `json:"imapPort"`
	IMAPUser string `json:"imapUser"`
	IMAPPass string `json:"imapPass"`
	SMTPHost string `json:"smtpHost"`
	SMTPPort int    `json:"smtpPort"`
	SMTPUser string `json:"smtpUser"`
	SMTPPass string `json:"smtpPass"`
	Address  string `json:"address"`
}

type EmailMessage struct {
	From        string
	To          string
	Subject     string
	Body        string
	MessageID   string
	InReplyTo   string
	SeqNum      uint32
	Attachments []EmailAttachment
	Date        time.Time
}

type EmailAttachment struct {
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}

type EmailBackend struct {
	cfg       EmailConfig
	lastSeq   uint32
	privateKey *crypto.Key
	publicKey  string
	keyRing    *crypto.KeyRing
	processed  map[string]bool
}

func NewEmailBackend(cfg EmailConfig, dataDir string) *EmailBackend {
	if cfg.IMAPPort == 0 { cfg.IMAPPort = 993 }
	if cfg.SMTPPort == 0 { cfg.SMTPPort = 465 }
	eb := &EmailBackend{cfg: cfg, processed: make(map[string]bool)}
	eb.initPGP(dataDir)
	return eb
}

func (e *EmailBackend) initPGP(dataDir string) {
	keyPath := filepath.Join(dataDir, "smago.key")
	if data, err := os.ReadFile(keyPath); err == nil {
		key, err := crypto.NewKeyFromArmored(string(data))
		if err == nil {
			e.privateKey = key
			ap, _ := key.GetArmoredPublicKey()
			e.publicKey = ap
			ring, err := crypto.NewKeyRing(key)
			if err == nil {
				e.keyRing = ring
				log.Printf("email: PGP key loaded (fp: %s)", key.GetFingerprint()[:16])
				return
			}
		}
		log.Printf("email: key load failed: %v, generating new", err)
	}
	log.Printf("email: generating PGP key for %s...", e.cfg.Address)
	key, err := crypto.GenerateKey("SMAGo", e.cfg.Address, "curve25519", 0)
	if err != nil {
		log.Printf("email: key gen failed: %v", err)
		return
	}
	_ = os.MkdirAll(dataDir, 0755)
	armored, _ := key.Armor()
	_ = os.WriteFile(keyPath, []byte(armored), 0600)
	e.privateKey = key
	ap, _ := key.GetArmoredPublicKey()
	e.publicKey = ap
	ring, _ := crypto.NewKeyRing(key)
	e.keyRing = ring
	log.Printf("email: PGP key generated (fp: %s)", key.GetFingerprint()[:16])
}

// decrypt tries to decrypt a PGP message. Returns original on failure.
func (e *EmailBackend) decrypt(armored string) (string, bool) {
	if e.keyRing == nil { return armored, false }
	msg, err := crypto.NewPGPMessageFromArmored(armored)
	if err != nil { return armored, false }
	decrypted, err := e.keyRing.Decrypt(msg, nil, 0)
	if err != nil { return armored, false }
	return string(decrypted.GetBinary()), true
}

// ── IMAP ──────────────────────────────────────────────

func (e *EmailBackend) connectIMAP() (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", e.cfg.IMAPHost, e.cfg.IMAPPort)
	var c *client.Client
	var err error
	if e.cfg.IMAPPort == 143 {
		c, err = client.Dial(addr)
	} else {
		c, err = client.DialTLS(addr, &tls.Config{ServerName: e.cfg.IMAPHost})
	}
	if err != nil { return nil, fmt.Errorf("imap connect: %w", err) }
	if err := c.Login(e.cfg.IMAPUser, e.cfg.IMAPPass); err != nil {
		c.Logout()
		return nil, fmt.Errorf("imap login: %w", err)
	}
	return c, nil
}

func (e *EmailBackend) Poll() ([]EmailMessage, error) {
	if e.cfg.IMAPHost == "" { return nil, nil }
	c, err := e.connectIMAP()
	if err != nil { return nil, err }
	defer c.Logout()

	mbox, err := c.Select("INBOX", true)
	if err != nil { return nil, fmt.Errorf("imap select: %w", err) }
	if mbox.Messages == 0 { return nil, nil }

	var fromSeq, toSeq uint32
	if e.lastSeq == 0 {
		// First run: skip to current position, don't process old mail
		e.lastSeq = mbox.Messages
		return nil, nil
	} else if e.lastSeq < mbox.Messages {
		fromSeq = e.lastSeq + 1
		toSeq = mbox.Messages
	} else {
		return nil, nil
	}

	var seqSet imap.SeqSet
	seqSet.AddRange(fromSeq, toSeq)

	fetchItems := []imap.FetchItem{imap.FetchEnvelope, imap.FetchItem("RFC822")}
	msgChan := make(chan *imap.Message, 10)
	fetchDone := make(chan error, 1)
	go func() { fetchDone <- c.Fetch(&seqSet, fetchItems, msgChan) }()

	var emails []EmailMessage
	for msg := range msgChan {
		if msg.Envelope == nil { continue }
		fromAddr := ""
		if len(msg.Envelope.From) > 0 {
			fromAddr = msg.Envelope.From[0].MailboxName + "@" + msg.Envelope.From[0].HostName
		}
		toAddr := ""
		if len(msg.Envelope.To) > 0 {
			toAddr = msg.Envelope.To[0].MailboxName + "@" + msg.Envelope.To[0].HostName
		}
		subject := decodeHeader(msg.Envelope.Subject)
		body := ""
		var attachments []EmailAttachment
		for _, lit := range msg.Body {
			raw, _ := io.ReadAll(lit)
			body, attachments = parseEmailBody(raw)
			// Import any attached PGP keys
			for _, att := range attachments {
				if att.ContentType == "application/pgp-keys" || strings.HasSuffix(att.Filename, ".asc") {
					e.importKey(string(att.Data))
				}
			}
			// Try decrypt PGP message
			if strings.Contains(body, "-----BEGIN PGP MESSAGE-----") {
				if dec, ok := e.decrypt(body); ok { body = dec }
			}
			break
		}
		em := EmailMessage{
			From: fromAddr, To: toAddr, Subject: subject, Body: body,
			MessageID: msg.Envelope.MessageId, Attachments: attachments,
			SeqNum: msg.SeqNum, Date: msg.Envelope.Date,
		}
		if len(msg.Envelope.InReplyTo) > 0 { em.InReplyTo = string(msg.Envelope.InReplyTo) }
		if !e.processed[em.MessageID] {
			e.processed[em.MessageID] = true
			// Loop detection: skip subjects with excessive Re: prefixes
			reCount := 0
			for s := em.Subject; strings.HasPrefix(s, "Re: "); s = strings.TrimPrefix(s, "Re: ") {
				reCount++
			}
			if reCount <= 5 {
				emails = append(emails, em)
			}
		}
	}
	if err := <-fetchDone; err != nil { return nil, fmt.Errorf("imap fetch: %w", err) }
	e.lastSeq = toSeq
	return emails, nil
}

// ── SMTP ──────────────────────────────────────────────

func (e *EmailBackend) SendMail(to, subject, body, inReplyTo string) error {
	if e.cfg.SMTPHost == "" { return fmt.Errorf("smtp not configured") }

	var buf bytes.Buffer
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("From: %s\r\n", e.cfg.Address))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", encodeHeader(subject)))
	buf.WriteString("Chat-Version: 1.0\r\n")
	buf.WriteString("Chat-User-Agent: SMAGo/1.0\r\n")
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))

	// Autocrypt header - keydata must be base64 of binary PUBLIC key
	if e.publicKey != "" {
		// De-armor the public key to get binary
		pubKey, err := crypto.NewKeyFromArmored(e.publicKey)
		var keyB64 string
		if err == nil {
			binKey, err2 := pubKey.Serialize()
			if err2 == nil {
				keyB64 = base64.StdEncoding.EncodeToString(binKey)
			}
		}
		if keyB64 == "" {
			// fallback: encode armored text
			keyB64 = base64.StdEncoding.EncodeToString([]byte(e.publicKey))
		}
		buf.WriteString(fmt.Sprintf("Autocrypt: addr=%s; prefer-encrypt=mutual; keydata=\r\n", e.cfg.Address))
		for i := 0; i < len(keyB64); i += 72 {
			end := i + 72
			if end > len(keyB64) { end = len(keyB64) }
			buf.WriteString(" " + keyB64[i:end] + "\r\n")
		}
	}

	if inReplyTo != "" {
		buf.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyTo))
		buf.WriteString(fmt.Sprintf("References: %s\r\n", inReplyTo))
	}

	buf.WriteString("Content-Type: text/plain; charset=utf-8; format=flowed\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&buf)
	qp.Write([]byte(body))
	qp.Close()

	// Send via SMTP
	addr := fmt.Sprintf("%s:%d", e.cfg.SMTPHost, e.cfg.SMTPPort)
	auth := smtp.PlainAuth("", e.cfg.SMTPUser, e.cfg.SMTPPass, e.cfg.SMTPHost)
	var smtpClient *smtp.Client
	if e.cfg.SMTPPort == 587 {
		conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
		if err != nil { return fmt.Errorf("smtp dial: %w", err) }
		smtpClient, err = smtp.NewClient(conn, e.cfg.SMTPHost)
		if err != nil { conn.Close(); return fmt.Errorf("smtp client: %w", err) }
		if ok, _ := smtpClient.Extension("STARTTLS"); ok {
			_ = smtpClient.StartTLS(&tls.Config{ServerName: e.cfg.SMTPHost})
		}
		defer smtpClient.Close()
	} else {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: e.cfg.SMTPHost})
		if err != nil { return fmt.Errorf("smtp dial: %w", err) }
		smtpClient, err = smtp.NewClient(conn, e.cfg.SMTPHost)
		if err != nil { conn.Close(); return fmt.Errorf("smtp client: %w", err) }
		defer smtpClient.Close()
	}
	if err := smtpClient.Auth(auth); err != nil { return fmt.Errorf("smtp auth: %w", err) }
	if err := smtpClient.Mail(e.cfg.Address); err != nil { return fmt.Errorf("smtp mail: %w", err) }
	if err := smtpClient.Rcpt(to); err != nil { return fmt.Errorf("smtp rcpt: %w", err) }
	w, err := smtpClient.Data()
	if err != nil { return fmt.Errorf("smtp data: %w", err) }
	_, err = w.Write(buf.Bytes())
	if err != nil { return fmt.Errorf("smtp write: %w", err) }
	return w.Close()
}

// ── Email parsing ─────────────────────────────────────

func parseEmailBody(raw []byte) (string, []EmailAttachment) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil { return string(raw), nil }
	ct := msg.Header.Get("Content-Type")
	var body string
	var attachments []EmailAttachment
	// Handle multipart/encrypted (Delta Chat E2E)
	if strings.HasPrefix(ct, "multipart/encrypted") {
		_, params, parseErr := mime.ParseMediaType(ct)
		if parseErr != nil { return "", nil }
		boundary := params["boundary"]
		if boundary == "" { return "", nil }
		mr := multipart.NewReader(bytes.NewReader(raw), boundary)
		for {
			part, partErr := mr.NextPart()
			if partErr != nil { break }
			partCT := part.Header.Get("Content-Type")
			if partCT == "application/octet-stream" || strings.Contains(partCT, "encrypted.asc") {
				data, _ := io.ReadAll(part)
				body = string(data)
			}
		}
		return body, attachments
	}
	if strings.HasPrefix(ct, "multipart/") {
		_, params, parseErr := mime.ParseMediaType(ct)
		if parseErr != nil { return string(raw), nil }
		boundary := params["boundary"]
		if boundary == "" { return string(raw), nil }
		mr := multipart.NewReader(bytes.NewReader(raw), boundary)
		for {
			part, partErr := mr.NextPart()
			if partErr != nil { break }
			partCT := part.Header.Get("Content-Type")
			partDisp := part.Header.Get("Content-Disposition")
			data, _ := io.ReadAll(part)
			if int64(len(data)) > emailMaxFileSize { continue }
			if strings.HasPrefix(partCT, "text/") {
				if body == "" {
					data = decodeTransferEncoding(data, part.Header.Get("Content-Transfer-Encoding"))
					body = string(data)
				}
			} else if strings.Contains(partDisp, "attachment") || strings.Contains(partDisp, "inline") {
				filename := extractFilename(partDisp, partCT)
				attachments = append(attachments, EmailAttachment{
					Filename: filename, ContentType: partCT, Size: int64(len(data)), Data: data,
				})
			}
		}
	} else {
		data, _ := io.ReadAll(msg.Body)
		data = decodeTransferEncoding(data, msg.Header.Get("Content-Transfer-Encoding"))
		body = string(data)
	}
	return strings.TrimSpace(body), attachments
}

func decodeTransferEncoding(data []byte, encoding string) []byte {
	switch strings.ToLower(encoding) {
	case "base64":
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(decoded, data)
		if err != nil { n, _ = base64.RawStdEncoding.Decode(decoded, data) }
		return decoded[:n]
	case "quoted-printable":
		decoded, _ := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(data)))
		return decoded
	default:
		return data
	}
}

func extractFilename(disp, ct string) string {
	_, params, _ := mime.ParseMediaType(disp)
	if name, ok := params["filename"]; ok { return name }
	_, params2, _ := mime.ParseMediaType(ct)
	if name, ok := params2["name"]; ok { return name }
	return "attachment"
}

func decodeHeader(s string) string {
	if s == "" { return "" }
	dec := new(mime.WordDecoder)
	d, err := dec.DecodeHeader(s)
	if err != nil { return s }
	return d
}

func encodeHeader(s string) string {
	if strings.ContainsAny(s, "=?\r\n") { return mime.QEncoding.Encode("utf-8", s) }
	return s
}


func (e *EmailBackend) importKey(armored string) {
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil { return }
	if e.keyRing == nil {
		ring, err := crypto.NewKeyRing(key)
		if err == nil {
			e.keyRing = ring
			log.Printf("email: imported key for %s (fp: %s)", key.GetEntity().PrimaryIdentity().UserId.Email, key.GetFingerprint()[:16])
		}
		return
	}
	_ = e.keyRing.AddKey(key)
	log.Printf("email: added key for %s (fp: %s)", key.GetEntity().PrimaryIdentity().UserId.Email, key.GetFingerprint()[:16])
}
