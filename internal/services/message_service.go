package services

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidMessage         = errors.New("invalid message")
	ErrInvalidMessageID       = errors.New("invalid message id")
	ErrInvalidSenderID        = errors.New("invalid sender id")
	ErrInvalidRecipientID     = errors.New("invalid recipient id")
	ErrInvalidMessageUser     = errors.New("invalid message user id")
	ErrRecipientNotFound      = errors.New("recipient not found")
	ErrRecipientNotActive     = errors.New("recipient is not active")
	ErrMessageSubjectRequired = errors.New("message subject is required")
	ErrMessageBodyRequired    = errors.New("message body is required")
	ErrMessageUserNotFound    = errors.New("message user not found")
)

type MessageService struct {
	messageRepo *repository.MessageRepository
	userRepo    *repository.UserRepository
}

func NewMessageService(
	messageRepo *repository.MessageRepository,
	userRepo *repository.UserRepository,
) *MessageService {
	return &MessageService{
		messageRepo: messageRepo,
		userRepo:    userRepo,
	}
}

func (s *MessageService) SendMessage(
	senderID string,
	recipientID string,
	subject string,
	body string,
) (*models.Message, error) {
	parsedSenderID, err := parseMessageUUID(senderID, ErrInvalidSenderID)
	if err != nil {
		return nil, err
	}

	parsedRecipientID, err := parseMessageUUID(recipientID, ErrInvalidRecipientID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(subject) == "" {
		return nil, ErrMessageSubjectRequired
	}

	if strings.TrimSpace(body) == "" {
		return nil, ErrMessageBodyRequired
	}

	if _, err := s.findActiveUser(parsedSenderID); err != nil {
		return nil, err
	}

	if _, err := s.findActiveRecipient(parsedRecipientID); err != nil {
		return nil, err
	}

	message := &models.Message{
		SenderID:    parsedSenderID,
		RecipientID: parsedRecipientID,
		Subject:     strings.TrimSpace(subject),
		Body:        strings.TrimSpace(body),
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *MessageService) GetMessage(
	id string,
	userID string,
) (*models.Message, error) {
	parsedID, err := parseMessageUUID(id, ErrInvalidMessageID)
	if err != nil {
		return nil, err
	}

	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return nil, err
	}

	return s.messageRepo.FindByID(parsedID, parsedUserID)
}

func (s *MessageService) ListInbox(
	userID string,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return nil, 0, err
	}

	return s.messageRepo.ListInbox(parsedUserID, page, pageSize)
}

func (s *MessageService) ListSent(
	userID string,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return nil, 0, err
	}

	return s.messageRepo.ListSent(parsedUserID, page, pageSize)
}

func (s *MessageService) ListUnread(
	userID string,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return nil, 0, err
	}

	return s.messageRepo.ListUnreadInbox(parsedUserID, page, pageSize)
}

func (s *MessageService) MarkAsRead(
	id string,
	userID string,
) error {
	parsedID, err := parseMessageUUID(id, ErrInvalidMessageID)
	if err != nil {
		return err
	}

	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return err
	}

	return s.messageRepo.MarkAsRead(parsedID, parsedUserID)
}

func (s *MessageService) DeleteMessage(
	id string,
	userID string,
) error {
	parsedID, err := parseMessageUUID(id, ErrInvalidMessageID)
	if err != nil {
		return err
	}

	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return err
	}

	return s.messageRepo.SoftDelete(parsedID, parsedUserID)
}

func (s *MessageService) GetUnreadCount(userID string) (int64, error) {
	parsedUserID, err := parseMessageUUID(userID, ErrInvalidMessageUser)
	if err != nil {
		return 0, err
	}

	return s.messageRepo.CountUnread(parsedUserID)
}

func (s *MessageService) findActiveUser(userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(userID.String())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrMessageUserNotFound
		}
		return nil, err
	}

	if user.Status != models.UserStatusActive {
		return nil, ErrMessageUserNotFound
	}

	return user, nil
}

func (s *MessageService) findActiveRecipient(userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(userID.String())
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrRecipientNotFound
	}
	if err != nil {
		return nil, err
	}

	if user.Status != models.UserStatusActive {
		return nil, ErrRecipientNotActive
	}

	return user, nil
}

func parseMessageUUID(value string, invalid error) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, invalid
	}

	return parsed, nil
}
