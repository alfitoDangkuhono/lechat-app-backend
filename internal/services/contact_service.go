package services

import (
	"math"
	"strings"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/repository"
)

type ContactService struct {
	contactRepository *repository.ContactRepository
}

func NewContactService(contactRepository *repository.ContactRepository) interfaces.IContacts {
	return &ContactService{contactRepository: contactRepository}
}

func (s *ContactService) RequestContactByPhone(userID uint, phone string) (uint, string, error) {
	phone = strings.TrimSpace(phone)
	if userID == 0 || phone == "" || len(phone) < 10 {
		return 0, "", constant.ErrChatInvalidRequest
	}
	return s.contactRepository.RequestContactByPhone(userID, phone)
}

func (s *ContactService) AcceptContact(userID, targetID uint) error {
	if !validContactPair(userID, targetID) {
		return constant.ErrChatInvalidRequest
	}
	return s.contactRepository.AcceptContact(userID, targetID)
}

func (s *ContactService) RejectContact(userID, targetID uint) error {
	if !validContactPair(userID, targetID) {
		return constant.ErrChatInvalidRequest
	}
	return s.contactRepository.RejectContact(userID, targetID)
}

func (s *ContactService) RemoveContact(userID, targetID uint) error {
	if !validContactPair(userID, targetID) {
		return constant.ErrChatInvalidRequest
	}
	return s.contactRepository.RemoveContact(userID, targetID)
}

func (s *ContactService) ListContacts(userID uint) ([]dto.ContactDto, error) {
	return s.contactRepository.ListContacts(userID)
}

func (s *ContactService) ListContactRequests(userID uint) ([]dto.ContactDto, error) {
	return s.contactRepository.ListContactRequests(userID)
}

func validContactPair(userID, targetID uint) bool {
	return userID > 0 && targetID > 0 && userID != targetID &&
		uint64(userID) <= math.MaxInt64 && uint64(targetID) <= math.MaxInt64
}
