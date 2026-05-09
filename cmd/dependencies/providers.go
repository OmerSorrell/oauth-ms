package dependencies

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/adapter/provider/github"
	"github.com/OmerSorrell/oauth-ms/internal/adapter/provider/registry"
	"github.com/OmerSorrell/oauth-ms/internal/config"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
)

// provideGithubProvider builds the GitHub strategy. To add Slack/Salesforce:
// add a sibling provideXProvider here and pass it into provideProviderRegistry.
func provideGithubProvider(_ *slog.Logger, cfg *config.Config) (provider.Provider, error) {
	httpc := &http.Client{Timeout: 10 * time.Second}
	return github.New(github.Config{
		ClientID:     cfg.Providers.GitHub.ClientID,
		ClientSecret: cfg.Providers.GitHub.ClientSecret,
		Scopes:       cfg.Providers.GitHub.Scopes,
	}, httpc)
}

func provideProviderRegistry(log *slog.Logger, provs ...provider.Provider) (provider.Registry, error) {
	reg, err := registry.New(provs...)
	if err != nil {
		return nil, err
	}
	log.Info("provider registry initialized", "providers", reg.Names())
	return reg, nil
}
