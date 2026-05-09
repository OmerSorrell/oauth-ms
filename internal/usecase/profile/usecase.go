// Package profile contains the /profile aggregation use case.
//
// The provider call and the internal-service call are independent and run in
// parallel via errgroup; either's error cancels the other.
package profile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	domauth "github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	domprofile "github.com/OmerSorrell/oauth-ms/internal/domain/profile"
	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
)

// UseCase composes a normalized profile from the provider and the internal service.
type UseCase struct {
	log         *slog.Logger
	registry    provider.Registry
	internalSvc internalsvc.InternalService
}

func New(log *slog.Logger, reg provider.Registry, svc internalsvc.InternalService) *UseCase {
	return &UseCase{log: log, registry: reg, internalSvc: svc}
}

// GetProfile fans out provider.ListResources and InternalService.GetUser concurrently.
//
// On error from either branch the errgroup ctx cancels the other in-flight call.
func (uc *UseCase) GetProfile(ctx context.Context, sess domauth.Session) (domprofile.NormalizedProfile, error) {
	p, err := uc.registry.Get(sess.Provider)
	if err != nil {
		return domprofile.NormalizedProfile{}, err
	}

	g, gctx := errgroup.WithContext(ctx)

	var (
		resources []domprofile.Resource
		user      domprofile.User
	)

	g.Go(func() error {
		token := provider.Token{
			AccessToken: sess.AccessToken,
			TokenType:   sess.TokenType,
			ExpiresAt:   sess.AccessTokenExpiresAt,
			Scopes:      sess.Scopes,
		}
		r, err := p.ListResources(gctx, token)
		if err != nil {
			return fmt.Errorf("profile: list resources: %w", err)
		}
		resources = r
		return nil
	})

	g.Go(func() error {
		u, err := uc.internalSvc.GetUser(gctx, sess.ProviderUserID)
		if err != nil {
			if errors.Is(err, internalsvc.ErrUserNotFound) {
				return err
			}
			return fmt.Errorf("profile: internal user: %w", err)
		}
		user = u
		return nil
	})

	if err := g.Wait(); err != nil {
		return domprofile.NormalizedProfile{}, err
	}
	return domprofile.NormalizedProfile{User: user, Resources: resources}, nil
}
