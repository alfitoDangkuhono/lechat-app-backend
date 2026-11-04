package interfaces

import "lechat-backend/internal/database/dto"

type IContacts interface {
	RequestContactByPhone(userID uint, phone string) (uint, string, error)
	AcceptContact(userID, targetID uint) error
	RejectContact(userID, targetID uint) error
	RemoveContact(userID, targetID uint) error
	ListContacts(userID uint) ([]dto.ContactDto, error)
	ListContactRequests(userID uint) ([]dto.ContactDto, error)
}
