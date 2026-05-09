// Package internalsvc defines the port for the internal user-enrichment service
// (/api/users/{id} with X-Internal-API-Key).
package internalsvc

import (
	"context"
	"errors"

	"github.com/OmerSorrell/oauth-ms/internal/domain/profile"
)

// ErrUserNotFound is returned when the internal service replies 404.
var ErrUserNotFound = errors.New("internalsvc: user not found")

// ErrUnauthorized is returned when the API key is missing/invalid (401/403).
var ErrUnauthorized = errors.New("internalsvc: unauthorized")

// InternalService fetches enriched user data from the internal users endpoint.
type InternalService interface {
	GetUser(ctx context.Context, userID string) (profile.User, error)
}
