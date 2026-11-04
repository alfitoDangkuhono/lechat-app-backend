package repository

import (
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"

	"gorm.io/gorm"
)

type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

func (r *ContactRepository) RequestContactByPhone(userID uint, phone string) (uint, string, error) {
	var targetIDs []uint
	if err := r.db.Model(&models.Users{}).Where("phone = ?", phone).Limit(2).Pluck("id", &targetIDs).Error; err != nil {
		return 0, "", err
	}
	if len(targetIDs) == 0 {
		return 0, "", gorm.ErrRecordNotFound
	}
	if len(targetIDs) != 1 {
		return 0, "", constant.ErrChatConflict
	}

	targetID := targetIDs[0]
	if userID == targetID || userID == 0 || targetID == 0 {
		return 0, "", constant.ErrChatInvalidRequest
	}

	low, high := orderedUsers(userID, targetID)
	var outcome models.ContactRequestOutcome
	err := r.db.Raw(`
		INSERT INTO contacts (user_low, user_high, requested_by)
		SELECT ?, ?, ?
		WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)
		ON CONFLICT (user_low, user_high)
		DO UPDATE SET
			status = CASE
				WHEN contacts.status = 'pending'
				 AND contacts.requested_by <> EXCLUDED.requested_by
				THEN 'accepted'
				ELSE contacts.status
			END,
			accepted_at = CASE
				WHEN contacts.status = 'pending'
				 AND contacts.requested_by <> EXCLUDED.requested_by
				THEN CURRENT_TIMESTAMP
				ELSE contacts.accepted_at
			END,
			updated_at = CASE
				WHEN contacts.status = 'pending'
				 AND contacts.requested_by <> EXCLUDED.requested_by
				THEN CURRENT_TIMESTAMP
				ELSE contacts.updated_at
			END
		RETURNING status, requested_by
	`, low, high, userID, targetID).Scan(&outcome).Error
	if err != nil {
		return 0, "", err
	}
	if outcome.Status == "" {
		return 0, "", gorm.ErrRecordNotFound
	}
	if outcome.Status == "pending" && outcome.RequestedBy == userID {
		return 0, "", constant.ErrContactRequestPending
	}
	return targetID, outcome.Status, nil
}

func (r *ContactRepository) AcceptContact(userID, targetID uint) error {
	if userID == targetID || userID == 0 || targetID == 0 {
		return constant.ErrChatInvalidRequest
	}
	low, high := orderedUsers(userID, targetID)
	result := r.db.Exec(`
		UPDATE contacts
		SET status = 'accepted',
		    accepted_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_low = ?
		  AND user_high = ?
		  AND status = 'pending'
		  AND requested_by = ?
		  AND requested_by <> ?
	`, low, high, targetID, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ContactRepository) RejectContact(userID, targetID uint) error {
	if userID == targetID || userID == 0 || targetID == 0 {
		return constant.ErrChatInvalidRequest
	}
	low, high := orderedUsers(userID, targetID)
	result := r.db.Exec(`
		DELETE FROM contacts
		WHERE user_low = ?
		  AND user_high = ?
		  AND status = 'pending'
		  AND requested_by = ?
		  AND requested_by <> ?
	`, low, high, targetID, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ContactRepository) RemoveContact(userID, targetID uint) error {
	if userID == targetID || userID == 0 || targetID == 0 {
		return constant.ErrChatInvalidRequest
	}
	low, high := orderedUsers(userID, targetID)
	result := r.db.Exec(`
		DELETE FROM contacts
		WHERE user_low = ?
		  AND user_high = ?
		  AND status = 'accepted'
	`, low, high)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ContactRepository) ListContacts(userID uint) ([]dto.ContactDto, error) {
	var contacts []dto.ContactDto
	err := r.db.Raw(`
		SELECT
			CASE WHEN contact.user_low = ? THEN contact.user_high ELSE contact.user_low END AS user_id,
			users.username,
			COALESCE(users.first_name, '') AS first_name,
			COALESCE(users.last_name, '') AS last_name,
			contact.status,
			contact.created_at,
			contact.accepted_at
		FROM contacts AS contact
		JOIN users
		  ON users.id = CASE
			WHEN contact.user_low = ? THEN contact.user_high
			ELSE contact.user_low
		  END
		WHERE (contact.user_low = ? OR contact.user_high = ?)
		  AND contact.status = 'accepted'
		ORDER BY COALESCE(users.first_name, users.username), users.id
	`, userID, userID, userID, userID).Scan(&contacts).Error
	return contacts, err
}

func (r *ContactRepository) ListContactRequests(userID uint) ([]dto.ContactDto, error) {
	var requests []dto.ContactDto
	err := r.db.Raw(`
		SELECT
			CASE WHEN contact.user_low = ? THEN contact.user_high ELSE contact.user_low END AS user_id,
			users.username,
			COALESCE(users.first_name, '') AS first_name,
			COALESCE(users.last_name, '') AS last_name,
			contact.status,
			CASE WHEN contact.requested_by = ? THEN 'outgoing' ELSE 'incoming' END AS direction,
			contact.created_at
		FROM contacts AS contact
		JOIN users
		  ON users.id = CASE
			WHEN contact.user_low = ? THEN contact.user_high
			ELSE contact.user_low
		  END
		WHERE (contact.user_low = ? OR contact.user_high = ?)
		  AND contact.status = 'pending'
		ORDER BY contact.created_at DESC
	`, userID, userID, userID, userID, userID).Scan(&requests).Error
	return requests, err
}

func orderedUsers(first, second uint) (uint, uint) {
	if first < second {
		return first, second
	}
	return second, first
}
