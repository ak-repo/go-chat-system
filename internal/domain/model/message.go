package model

import (
	"database/sql"
	"time"
)

// DAO
type Message struct {
	ID                     string              `json:"id" db:"id"`
	SenderID               string              `json:"sender_id" db:"sender_id"`
	ReceiverID             string              `json:"receiver_id" db:"receiver_id"`
	Body                   string              `json:"content" db:"body"`
	IsGroup                bool                `json:"is_group" db:"is_group"`
	CreatedAt              time.Time           `json:"created_at" db:"created_at"`
	ModifiedAt             time.Time           `json:"modified_at,omitempty" db:"modified_at" `
	DeletedAt              sql.NullTime        `json:"deleted_at,omitempty" db:"deleted_at" `
	ConversationID         string              `json:"conversation_id,omitempty" db:"conversation_id"`
	ClientMessageID        string              `json:"client_message_id,omitempty" db:"client_message_id"`
	EditedAt               *time.Time          `json:"edited_at,omitempty" db:"edited_at"`
	ReplyToMessageID       string              `json:"reply_to_message_id,omitempty" db:"reply_to_message_id"`
	ForwardedFromMessageID string              `json:"forwarded_from_message_id,omitempty" db:"forwarded_from_message_id"`
	ReplyTo                *MessagePreview     `json:"reply_to,omitempty"`
	ForwardedFrom          *MessagePreview     `json:"forwarded_from,omitempty"`
	Reactions              []ReactionAggregate `json:"reactions"`
	Status                 string              `json:"status,omitempty" db:"status"`
	Mentions               []MessageMention    `json:"mentions"`
	CreatedNotifications   []Notification      `json:"-"`
}

type MessageMention struct {
	Kind   string `json:"kind"`
	UserID string `json:"user_id,omitempty"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type Notification struct {
	ID             string         `json:"id"`
	RecipientID    string         `json:"recipient_id"`
	ActorID        string         `json:"actor_id"`
	Type           string         `json:"type"`
	ConversationID string         `json:"conversation_id"`
	MessageID      string         `json:"message_id"`
	Payload        map[string]any `json:"payload"`
	CreatedAt      time.Time      `json:"created_at"`
	ReadAt         *time.Time     `json:"read_at,omitempty"`
}

type NotificationPreferences struct {
	MessageEnabled bool `json:"message_enabled"`
	ReplyEnabled   bool `json:"reply_enabled"`
	MentionEnabled bool `json:"mention_enabled"`
}

type MessagePreview struct {
	ID       string `json:"id"`
	SenderID string `json:"sender_id"`
	Content  string `json:"content"`
}

type ReactionAggregate struct {
	Reaction    string `json:"reaction"`
	Count       int    `json:"count"`
	ReactedByMe bool   `json:"reacted_by_me"`
}

type Messages []*Message

type ReadReceipt struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	SenderID       string `json:"sender_id"`
}

type Conversation struct {
	ID           string             `json:"id"`
	Kind         string             `json:"kind"`
	UserOneID    string             `json:"user_one_id,omitempty"`
	UserTwoID    string             `json:"user_two_id,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	ModifiedAt   time.Time          `json:"modified_at"`
	Archived     bool               `json:"archived"`
	Pinned       bool               `json:"pinned"`
	Muted        bool               `json:"muted"`
	MutedUntil   *time.Time         `json:"muted_until,omitempty"`
	Name         string             `json:"name,omitempty"`
	Description  string             `json:"description,omitempty"`
	CreatorID    string             `json:"creator_id,omitempty"`
	AvatarURL    string             `json:"avatar_url,omitempty"`
	MemberCount  int                `json:"member_count,omitempty"`
	CurrentRole  string             `json:"current_role,omitempty"`
	Capabilities *GroupCapabilities `json:"capabilities,omitempty"`
}

const (
	GroupRoleOwner  = "owner"
	GroupRoleAdmin  = "admin"
	GroupRoleMember = "member"
)

type GroupCapabilities struct {
	EditGroup     bool `json:"edit_group"`
	AddMembers    bool `json:"add_members"`
	RemoveMembers bool `json:"remove_members"`
	ManageRoles   bool `json:"manage_roles"`
	ManageInvites bool `json:"manage_invites"`
}

type GroupMember struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	JoinedAt   time.Time `json:"joined_at"`
	AddedBy    string    `json:"added_by,omitempty"`
	ModifiedAt time.Time `json:"modified_at"`
}

type GroupInvite struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	CreatorID      string     `json:"creator_id"`
	ExpiresAt      time.Time  `json:"expires_at"`
	MaxUses        *int       `json:"max_uses,omitempty"`
	UseCount       int        `json:"use_count"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
