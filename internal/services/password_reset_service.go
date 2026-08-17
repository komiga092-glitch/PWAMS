package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services/email"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

type PasswordResetService struct {
	passwordResetRepo *repository.PasswordResetRepository
	userRepo          *repository.UserRepository
	emailService      *email.EmailService
}

func NewPasswordResetService(
	passwordResetRepo *repository.PasswordResetRepository,
	userRepo *repository.UserRepository,
	emailService *email.EmailService,
) *PasswordResetService {
	return &PasswordResetService{
		passwordResetRepo: passwordResetRepo,
		userRepo:          userRepo,
		emailService:      emailService,
	}
}

func (s *PasswordResetService) ForgotPassword(email string) error {
	user, err := s.userRepo.FindByLogin(email)
	if err != nil {
		return err
	}

	// Remove expired reset tokens for this user.
	if err := s.passwordResetRepo.DeleteExpired(user.ID); err != nil {
		return fmt.Errorf("failed to clean expired reset tokens: %w", err)
	}

	// Generate a new 6-digit OTP.
	otp, err := utils.GenerateOTP()
	if err != nil {
		return fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Create a new password reset token.
	token := &models.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		OTP:       otp,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
		Verified:  false,
	}

	if err := s.passwordResetRepo.Create(token); err != nil {
		return fmt.Errorf("failed to create password reset token: %w", err)
	}

	subject := "PWAMS Password Reset OTP"

	body := fmt.Sprintf(
		"Your PWAMS password reset OTP is: %s\n\nThis OTP will expire in 10 minutes.",
		otp,
	)

	if err := s.emailService.Send(
		user.Email,
		subject,
		body,
	); err != nil {
		return fmt.Errorf("failed to send password reset OTP: %w", err)
	}

	return nil
}

func (s *PasswordResetService) VerifyResetOTP(
	email string,
	otp string,
) error {
	user, err := s.userRepo.FindByLogin(email)
	if err != nil {
		return err
	}

	token, err := s.passwordResetRepo.GetValidOTP(user.ID, otp)
	if err != nil {
		return fmt.Errorf("invalid or expired OTP")
	}

	if err := s.passwordResetRepo.MarkAsVerified(token.ID); err != nil {
		return fmt.Errorf("failed to verify OTP: %w", err)
	}

	return nil
}
func (s *PasswordResetService) ResetPassword(
	email, otp, newPassword, confirmPassword string,
) error {
	if newPassword != confirmPassword {
		return fmt.Errorf("new password and confirm password do not match")
	}

	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	user, err := s.userRepo.FindByLogin(email)
	if err != nil {
		return err
	}

	token, err := s.passwordResetRepo.GetVerifiedOTP(
		user.ID,
		otp,
	)
	if err != nil {
		return fmt.Errorf("invalid or expired OTP")
	}

	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(
		user.ID.String(),
		passwordHash,
	); err != nil {
		return err
	}

	if err := s.passwordResetRepo.ConsumeOTP(token.ID); err != nil {
		return fmt.Errorf("failed to consume OTP: %w", err)
	}

	return nil
}
