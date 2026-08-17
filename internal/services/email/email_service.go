package email

import (
	"fmt"
	"net/smtp"

	"github.com/komiga092-glitch/pwams/internal/config"
)

type EmailService struct {
	config *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		config: cfg,
	}
}

func (s *EmailService) Send(
	to string,
	subject string,
	body string,
) error {
	if s.config.SMTPHost == "" ||
		s.config.SMTPUsername == "" ||
		s.config.SMTPPassword == "" ||
		s.config.SMTPFrom == "" {
		return fmt.Errorf("SMTP configuration is missing")
	}

	auth := smtp.PlainAuth(
		"",
		s.config.SMTPUsername,
		s.config.SMTPPassword,
		s.config.SMTPHost,
	)

	message := []byte(
		"From: " + s.config.SMTPFrom + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body,
	)

	address := s.config.SMTPHost + ":" + s.config.SMTPPort

	if err := smtp.SendMail(
		address,
		auth,
		s.config.SMTPFrom,
		[]string{to},
		message,
	); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
