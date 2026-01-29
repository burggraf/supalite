package mailcapture

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/markb/supalite/internal/log"
	"github.com/markb/supalite/internal/pg"
)

// smtpBackend implements smtp.Backend
type smtpBackend struct {
	database *pg.EmbeddedDatabase
}

func (b *smtpBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &smtpSession{database: b.database}, nil
}

// smtpSession handles a single SMTP session
type smtpSession struct {
	database *pg.EmbeddedDatabase
	from     string
	to       []string
}

func (s *smtpSession) AuthPlain(username, password string) error {
	// Accept any auth for capture mode
	return nil
}

func (s *smtpSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *smtpSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *smtpSession) Data(r io.Reader) error {
	// Read the full message
	rawMessage, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Parse the message
	msg, err := mail.ReadMessage(bytes.NewReader(rawMessage))
	if err != nil {
		log.Warn("failed to parse email", "error", err)
		// Still store it even if parsing fails
		return s.storeEmail("", "", "", "", rawMessage)
	}

	subject := msg.Header.Get("Subject")

	// Decode subject if MIME encoded
	if decoded, err := decodeRFC2047(subject); err == nil {
		subject = decoded
	}

	// Extract body
	textBody, htmlBody := extractBodies(msg)

	// Store for each recipient
	for _, to := range s.to {
		if err := s.storeEmail(subject, textBody, htmlBody, to, rawMessage); err != nil {
			log.Warn("failed to store email", "error", err, "to", to)
		}
	}

	log.Info("captured email", "from", s.from, "to", s.to, "subject", subject)
	return nil
}

func (s *smtpSession) Reset() {
	s.from = ""
	s.to = nil
}

func (s *smtpSession) Logout() error {
	return nil
}

func (s *smtpSession) storeEmail(subject, textBody, htmlBody, to string, rawMessage []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.database.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		INSERT INTO public.captured_emails
			(from_addr, to_addr, subject, text_body, html_body, raw_message)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, s.from, to, subject, textBody, htmlBody, rawMessage)

	return err
}

// decodeRFC2047 decodes MIME encoded-word strings
func decodeRFC2047(s string) (string, error) {
	dec := new(mime.WordDecoder)
	return dec.DecodeHeader(s)
}

// extractBodies extracts text and HTML bodies from an email
func extractBodies(msg *mail.Message) (textBody, htmlBody string) {
	contentType := msg.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/") {
		// Handle multipart message
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return readBody(msg.Body), ""
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			boundary := params["boundary"]
			mr := multipart.NewReader(msg.Body, boundary)

			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}

				partContentType := part.Header.Get("Content-Type")
				body, _ := io.ReadAll(part)

				if strings.HasPrefix(partContentType, "text/plain") {
					textBody = string(body)
				} else if strings.HasPrefix(partContentType, "text/html") {
					htmlBody = string(body)
				}
			}
		}
	} else if strings.HasPrefix(contentType, "text/html") {
		htmlBody = readBody(msg.Body)
	} else {
		textBody = readBody(msg.Body)
	}

	return textBody, htmlBody
}

func readBody(r io.Reader) string {
	body, _ := io.ReadAll(r)
	return string(body)
}
