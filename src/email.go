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

// ── Config ────────────────────────────────────────────

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

// ── Message types ─────────────────────────────────────

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

// ── Backend ───────────────────────────────────────────

type EmailBackend struct {
	cfg        EmailConfig
	lastSeq    uint32
	privateKey *crypto.Key
	publicKey  string
	keyRing    *crypto.KeyRing
}

func NewEmailBackend(cfg EmailConfig, dataDir string) *EmailBackend {
	if cfg.IMAPPort == 0 {
		cfg.IMAPPort = 993
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 465
	}
	eb := &EmailBackend{cfg: cfg}
	eb.initPGP(dataDir)
	return eb
}

// ── OpenPGP ───────────────────────────────────────────

func (e *EmailBackend) initPGP(dataDir string) {
	keyPath := filepath.Join(dataDir, "smago.key")

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil {
		key, err := crypto.NewKeyFromArmored(string(data))
		if err == nil {
			e.privateKey = key
			ap, _ := key.GetArmoredPublicKey()
			e.publicKey = ap
			ring, err := crypto.NewKeyRing(key)
			if err == nil {
				e.keyRing = ring
				log.Printf("email: PGP key loaded (fingerprint: %s)", key.GetFingerprint())
				return
			}
		}
		log.Printf("email: failed to load key: %v, generating new", err)
	}

	// Generate new key
	log.Printf("email: generating PGP key for %s...", e.cfg.Address)
	key, err := crypto.GenerateKey("SMAGo", e.cfg.Address, "curve25519", 0)
	if err != nil {
		log.Printf("email: failed to generate key: %v", err)
		return
	}
	armored, err := key.Armor()
	if err != nil {
		log.Printf("email: failed to armor key: %v", err)
		return
	}
	if err := os.MkdirAll(dataDir, 0755); err == nil {
		_ = os.WriteFile(keyPath, []byte(armored), 0600)
	}
	e.privateKey = key
	armoredPub, _ := key.GetArmoredPublicKey()
	e.publicKey = armoredPub
	ring, err := crypto.NewKeyRing(key)
	if err != nil {
		log.Printf("email: failed to create keyring: %v", err)
		return
	}
	e.keyRing = ring
	log.Printf("email: PGP key generated (fingerprint: %s)", key.GetFingerprint())
}

func (e *EmailBackend) signMessage(plain string) string {
	if e.keyRing == nil {
		return plain
	}
	msg := crypto.NewPlainMessageFromString(plain)
	// Encrypt to self (sign only, no real encryption)
	encrypted, err := e.keyRing.Encrypt(msg, e.keyRing)
	if err != nil {
		log.Printf("email: sign failed: %v", err)
		return plain
	}
	armored, err := encrypted.GetArmored()
	if err != nil {
		return plain
	}
	return armored
}

func (e *EmailBackend) decryptMessage(armored string) (string, bool) {
	if e.keyRing == nil {
		return armored, false
	}
	msg, err := crypto.NewPGPMessageFromArmored(armored)
	if err != nil {
		return armored, false // not a PGP message
	}
	decrypted, err := e.keyRing.Decrypt(msg, nil, 0)
	if err != nil {
		log.Printf("email: decrypt failed: %v", err)
		return armored, false
	}
	return string(decrypted.GetBinary()), true
}

func (e *EmailBackend) PublicKeyArmored() string {
	return e.publicKey
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
	if err != nil {
		return nil, fmt.Errorf("imap connect: %w", err)
	}
	if err := c.Login(e.cfg.IMAPUser, e.cfg.IMAPPass); err != nil {
		c.Logout()
		return nil, fmt.Errorf("imap login: %w", err)
	}
	return c, nil
}

func (e *EmailBackend) Poll() ([]EmailMessage, error) {
	if e.cfg.IMAPHost == "" {
		return nil, nil
	}
	c, err := e.connectIMAP()
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	mbox, err := c.Select("INBOX", true)
	if err != nil {
		return nil, fmt.Errorf("imap select: %w", err)
	}
	if mbox.Messages == 0 {
		return nil, nil
	}

	var fromSeq, toSeq uint32
	if e.lastSeq == 0 {
		if mbox.Messages > 10 {
			fromSeq = mbox.Messages - 9
		} else {
			fromSeq = 1
		}
		toSeq = mbox.Messages
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
	go func() {
		fetchDone <- c.Fetch(&seqSet, fetchItems, msgChan)
	}()

	var emails []EmailMessage
	for msg := range msgChan {
		if msg.Envelope == nil {
			continue
		}
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
			break
		}

		em := EmailMessage{
			From:        fromAddr,
			To:          toAddr,
			Subject:     subject,
			Body:        body,
			MessageID:   msg.Envelope.MessageId,
			Attachments: attachments,
			SeqNum:      msg.SeqNum,
			Date:        msg.Envelope.Date,
		}
		if len(msg.Envelope.InReplyTo) > 0 {
			em.InReplyTo = string(msg.Envelope.InReplyTo)
		}
		emails = append(emails, em)
	}
	if err := <-fetchDone; err != nil {
		return nil, fmt.Errorf("imap fetch: %w", err)
	}
	e.lastSeq = toSeq
	return emails, nil
}

// ── SMTP ──────────────────────────────────────────────

func (e *EmailBackend) SendMail(to, subject, body, inReplyTo string) error {
	if e.cfg.SMTPHost == "" {
		return fmt.Errorf("smtp not configured")
	}

	var buf bytes.Buffer

	// Sign body with PGP
	signedBody := e.signMessage(body)

	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("From: %s\r\n", e.cfg.Address))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", encodeHeader(subject)))
	buf.WriteString("Chat-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/markdown; charset=utf-8; format=flowed\r\n")
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))

	// Autocrypt key in headers
	if pk := e.PublicKeyArmored(); pk != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte(pk))
		// Wrap long lines for Autocrypt header
		for i := 0; i < len(b64); i += 72 {
			end := i + 72
			if end > len(b64) {
				end = len(b64)
			}
			if i == 0 {
				buf.WriteString(fmt.Sprintf("Autocrypt: addr=%s; prefer-encrypt=mutual; keydata=\r\n %s\r\n", e.cfg.Address, b64[i:end]))
			} else {
				buf.WriteString(" " + b64[i:end] + "\r\n")
			}
		}
	}

	if inReplyTo != "" {
		buf.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyTo))
		buf.WriteString(fmt.Sprintf("References: %s\r\n", inReplyTo))
	}
	buf.WriteString("\r\n")
	buf.WriteString(signedBody)

	addr := fmt.Sprintf("%s:%d", e.cfg.SMTPHost, e.cfg.SMTPPort)
	auth := smtp.PlainAuth("", e.cfg.SMTPUser, e.cfg.SMTPPass, e.cfg.SMTPHost)
	var smtpClient *smtp.Client
	if e.cfg.SMTPPort == 587 {
		conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		smtpClient, err = smtp.NewClient(conn, e.cfg.SMTPHost)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		if ok, _ := smtpClient.Extension("STARTTLS"); ok {
			_ = smtpClient.StartTLS(&tls.Config{ServerName: e.cfg.SMTPHost})
		}
		defer smtpClient.Close()
	} else {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: e.cfg.SMTPHost})
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		smtpClient, err = smtp.NewClient(conn, e.cfg.SMTPHost)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		defer smtpClient.Close()
	}
	if err := smtpClient.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := smtpClient.Mail(e.cfg.Address); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := smtpClient.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := smtpClient.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	_, err = w.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return w.Close()
}

// ── Email parsing ─────────────────────────────────────

func parseEmailBody(raw []byte) (string, []EmailAttachment) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return string(raw), nil
	}
	ct := msg.Header.Get("Content-Type")
	var body string
	var attachments []EmailAttachment
	if strings.HasPrefix(ct, "multipart/") {
		_, params, parseErr := mime.ParseMediaType(ct)
		if parseErr != nil {
			return string(raw), nil
		}
		boundary := params["boundary"]
		if boundary == "" {
			return string(raw), nil
		}
		mr := multipart.NewReader(bytes.NewReader(raw), boundary)
		for {
			part, partErr := mr.NextPart()
			if partErr != nil {
				break
			}
			partCT := part.Header.Get("Content-Type")
			partDisp := part.Header.Get("Content-Disposition")
			data, _ := io.ReadAll(part)
			if int64(len(data)) > emailMaxFileSize {
				continue
			}
			if strings.HasPrefix(partCT, "text/plain") || strings.HasPrefix(partCT, "text/markdown") || strings.HasPrefix(partCT, "text/html") {
				if body == "" {
					data = decodeTransferEncoding(data, part.Header.Get("Content-Transfer-Encoding"))
					body = string(data)
				}
			} else if strings.Contains(partDisp, "attachment") || strings.Contains(partDisp, "inline") {
				filename := extractFilename(partDisp, partCT)
				attachments = append(attachments, EmailAttachment{
					Filename:    filename,
					ContentType: partCT,
					Size:        int64(len(data)),
					Data:        data,
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
		if err != nil {
			n, _ = base64.RawStdEncoding.Decode(decoded, data)
		}
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
	if name, ok := params["filename"]; ok {
		return name
	}
	_, params2, _ := mime.ParseMediaType(ct)
	if name, ok := params2["name"]; ok {
		return name
	}
	return "attachment"
}

func decodeHeader(s string) string {
	if s == "" {
		return ""
	}
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

func encodeHeader(s string) string {
	if strings.ContainsAny(s, "=?\r\n") {
		return mime.QEncoding.Encode("utf-8", s)
	}
	return s
}
