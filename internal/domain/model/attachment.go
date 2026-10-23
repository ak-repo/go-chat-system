package model

import "time"

// Attachment exposes validated media metadata, never bucket keys or credentials.
type Attachment struct {
	ID              string    `json:"id"`
	Filename        string    `json:"filename"`
	MIMEType        string    `json:"mime_type"`
	Size            int64     `json:"size_bytes"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	State           string    `json:"state"`
	Error           string    `json:"error,omitempty"`
	OwnerID         string    `json:"-"`
	ConversationID  string    `json:"-"`
	DeclaredMIME    string    `json:"-"`
	DeclaredSize    int64     `json:"-"`
	StagingKey      string    `json:"-"`
	FullKey         string    `json:"-"`
	ThumbnailKey    string    `json:"-"`
	UploadExpiresAt time.Time `json:"-"`
	Attempts        int       `json:"-"`
}
