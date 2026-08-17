package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services/email"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

type AccountActivationService struct {
	activationRepo *repository.AccountActivationRepository
	userRepo       *repository.UserRepository
	emailService   *email.EmailService
}

func NewAccountActivationService(
	activationRepo *repository.AccountActivationRepository,
	userRepo *repository.UserRepository,
	emailService *email.EmailService,
) *AccountActivationService {
	return &AccountActivationService{
		activationRepo: activationRepo,
		userRepo:       userRepo,
		emailService:   emailService,
	}
}

func (s *AccountActivationService) RequestActivation(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if strings.EqualFold(user.Status, "Active") {
		return fmt.Errorf("account is already active")
	}

	if err := s.activationRepo.DeleteExpired(user.ID); err != nil {
		return fmt.Errorf("failed to clean expired activation tokens: %w", err)
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return fmt.Errorf("failed to generate activation OTP: %w", err)
	}

	token := &models.AccountActivationToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		OTP:       otp,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
		Verified:  false,
	}

	if err := s.activationRepo.Create(token); err != nil {
		return fmt.Errorf("failed to create activation token: %w", err)
	}

	subject := "PWAMS Account Reactivation OTP"

	body := fmt.Sprintf(
		"Your PWAMS account reactivation OTP is: %s\n\nThis OTP will expire in 10 minutes.",
		otp,
	)

	if err := s.emailService.Send(
		user.Email,
		subject,
		body,
	); err != nil {
		return fmt.Errorf("failed to send activation OTP: %w", err)
	}

	return nil
}

func (s *AccountActivationService) VerifyActivationOTP(
	email string,
	otp string,
) error {
	email = strings.ToLower(strings.TrimSpace(email))
	otp = strings.TrimSpace(otp)

	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	token, err := s.activationRepo.GetValidOTP(
		user.ID,
		otp,
	)
	if err != nil {
		return fmt.Errorf("invalid or expired activation OTP")
	}

	if err := s.activationRepo.MarkAsVerified(token.ID); err != nil {
		return fmt.Errorf("failed to verify activation OTP: %w", err)
	}

	return nil
}

func (s *AccountActivationService) ReactivateAccount(
	email string,
	otp string,
) error {
	email = strings.ToLower(strings.TrimSpace(email))
	otp = strings.TrimSpace(otp)

	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	token, err := s.activationRepo.GetVerifiedOTP(
		user.ID,
		otp,
	)
	if err != nil {
		return fmt.Errorf("invalid or expired activation OTP")
	}

	if err := s.userRepo.UpdateStatus(
		user.ID.String(),
		"Active",
	); err != nil {
		return fmt.Errorf("failed to activate account: %w", err)
	}

	if err := s.activationRepo.ConsumeOTP(token.ID); err != nil {
		return fmt.Errorf("failed to consume activation OTP: %w", err)
	}

	return nil
}
