package models

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestChatQueryModelsMapDatabaseColumns(t *testing.T) {
	tests := []struct {
		name    string
		model   interface{}
		columns []string
	}{
		{
			name:  "conversation participant",
			model: ConversationParticipant{},
			columns: []string{
				"conversation_id", "user_id", "username", "first_name", "last_name", "role",
			},
		},
		{
			name:  "group invitation reference",
			model: GroupInvitationReference{},
			columns: []string{
				"id", "conversation_id", "inviter_id", "invitee_id",
			},
		},
		{
			name:    "conversation member role",
			model:   ConversationMemberRole{},
			columns: []string{"role"},
		},
		{
			name:    "conversation role",
			model:   ConversationRole{},
			columns: []string{"type", "role"},
		},
		{
			name:    "message mutation state",
			model:   MessageMutationState{},
			columns: []string{"sender_id", "deleted_at"},
		},
		{
			name:    "message conversation reference",
			model:   MessageConversationReference{},
			columns: []string{"conversation_id"},
		},
		{
			name:    "conversation access",
			model:   ConversationAccess{},
			columns: []string{"type", "contact_is_valid"},
		},
		{
			name:    "contact request outcome",
			model:   ContactRequestOutcome{},
			columns: []string{"status", "requested_by"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelSchema, err := schema.Parse(tt.model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatalf("parse GORM schema: %v", err)
			}
			if len(modelSchema.Fields) != len(tt.columns) {
				t.Fatalf("field count = %d, want %d", len(modelSchema.Fields), len(tt.columns))
			}

			columns := make(map[string]bool, len(modelSchema.Fields))
			for _, field := range modelSchema.Fields {
				columns[field.DBName] = true
			}
			for _, column := range tt.columns {
				if !columns[column] {
					t.Errorf("database column %q is not mapped", column)
				}
			}
		})
	}
}
