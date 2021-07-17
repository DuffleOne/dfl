package short

import (
	"bytes"
	"context"
	"time"
)

type Service interface {
	AddShortcut(context.Context, *ChangeShortcutRequest) error
	CreateSignedURL(context.Context, *CreateSignedURLRequest) (*CreateSignedURLResponse, error)
	DeleteResource(context.Context, *IdentifyResource) error
	ListResources(context.Context, *ListResourcesRequest) ([]*Resource, error)
	RemoveShortcut(context.Context, *ChangeShortcutRequest) error
	SetNSFW(context.Context, *SetNSFWRequest) error
	ShortenURL(context.Context, *CreateURLRequest) (*CreateResourceResponse, error)
	ViewDetails(context.Context, *IdentifyResource) (*Resource, error)
}

type Resource struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Serial    int        `json:"-"`
	Hash      *string    `json:"hash"`
	Name      *string    `json:"name"`
	OwnerID   string     `json:"owner_id"`
	Link      string     `json:"link"`
	NSFW      bool       `json:"nsfw"`
	MimeType  *string    `json:"mime_type"`
	Shortcuts []string   `json:"shortcuts"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

type ShortFormResource struct {
	ID     string
	Serial int
}

type CreateFileRequest struct {
	File bytes.Buffer `json:"file"`
	Name *string      `json:"name"`
}

type CreateURLRequest struct {
	URL string `json:"url"`
}

type CreateResourceResponse struct {
	ResourceID string `json:"resource_id"`
	Type       string `json:"type"`
	Hash       string `json:"hash"`
	URL        string `json:"url"`
}

type CreateSignedURLRequest struct {
	Name        *string `json:"name"`
	ContentType string  `json:"content_type"`
}

type CreateSignedURLResponse struct {
	ResourceID string  `json:"resource_id"`
	Type       string  `json:"type"`
	Name       *string `json:"name"`
	Hash       string  `json:"hash"`
	URL        string  `json:"url"`
	SignedLink string  `json:"link"`
}

type IdentifyResource struct {
	Query string `json:"query"`
}

type SetNSFWRequest struct {
	IdentifyResource
	NSFW bool `json:"nsfw"`
}

type ChangeShortcutRequest struct {
	IdentifyResource
	Shortcut string `json:"shortcut"`
}

type ListResourcesRequest struct {
	IncludeDeleted bool    `json:"include_deleted"`
	OwnerID        *string `json:"owner_id"`
	Limit          *uint64 `json:"limit"`
	FilterMime     *string `json:"filter_mime"`
}
