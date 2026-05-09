// Package config loads layered configuration into a typed Config via knadh/koanf.
//
// Precedence (lowest → highest): defaults < yaml file < env. Yaml is read from the
// first existing path among (extraPaths..., ./config/config.yaml, /etc/oauth-ms/config.yaml).
//
// Env keys are OAUTH_MS_<SECTION>_<KEY>. Section prefixes (server, auth, internal,
// providers.github, …) are translated into dotted koanf paths; the rest of the env
// name keeps its underscores. So OAUTH_MS_AUTH_STATE_TTL → auth.state_ttl, and
// OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID → providers.github.client_id.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	koanfenv "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the root configuration tree.
type Config struct {
	Server    Server    `koanf:"server"`
	Cache     Cache     `koanf:"cache"`
	Auth      Auth      `koanf:"auth"`
	Internal  Internal  `koanf:"internal"`
	Providers Providers `koanf:"providers"`
	Logging   Logging   `koanf:"logging"`
}

type Server struct {
	Addr            string        `koanf:"addr"`
	BaseURL         string        `koanf:"base_url"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

type Cache struct {
	Driver string `koanf:"driver"` // "memory" (Redis stub planned)
}

type Auth struct {
	StateTTL       time.Duration `koanf:"state_ttl"`
	SessionTTL     time.Duration `koanf:"session_ttl"`
	CookieSecure   bool          `koanf:"cookie_secure"`
	CookieSameSite string        `koanf:"cookie_samesite"` // "lax" | "strict" | "none"
	CookieDomain   string        `koanf:"cookie_domain"`
	PostLoginPath  string        `koanf:"post_login_path"`
}

type Internal struct {
	BaseURL     string        `koanf:"base_url"`
	APIKey      string        `koanf:"api_key"`
	HTTPTimeout time.Duration `koanf:"http_timeout"`
	CacheTTL    time.Duration `koanf:"cache_ttl"`
}

type Providers struct {
	GitHub GitHub `koanf:"github"`
}

type GitHub struct {
	ClientID     string   `koanf:"client_id"`
	ClientSecret string   `koanf:"client_secret"`
	Scopes       []string `koanf:"scopes"`
}

type Logging struct {
	Level  string `koanf:"level"`  // debug | info | warn | error
	Format string `koanf:"format"` // json | text
}

const envPrefix = "OAUTH_MS_"

// envSections maps env-name section prefixes (post-OAUTH_MS_ strip, longest first)
// to dotted koanf paths. Add an entry here when you introduce a nested section.
var envSections = []struct {
	envPrefix string
	koanfPath string
}{
	{"PROVIDERS_GITHUB_", "providers.github."},
	{"SERVER_", "server."},
	{"CACHE_", "cache."},
	{"AUTH_", "auth."},
	{"INTERNAL_", "internal."},
	{"LOGGING_", "logging."},
}

// Load reads defaults → yaml file → env into a Config and validates it.
func Load(extraPaths ...string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return nil, fmt.Errorf("config: defaults: %w", err)
	}

	paths := append([]string{}, extraPaths...)
	paths = append(paths, "./config/config.yaml", "/etc/oauth-ms/config.yaml")
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if err := k.Load(file.Provider(p), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("config: yaml %s: %w", p, err)
			}
			break
		}
	}

	if err := k.Load(koanfenv.Provider(".", koanfenv.Opt{
		Prefix:        envPrefix,
		TransformFunc: envTransform,
	}), nil); err != nil {
		return nil, fmt.Errorf("config: env: %w", err)
	}

	var c Config
	if err := k.Unmarshal("", &c); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// envTransform translates OAUTH_MS_AUTH_STATE_TTL → auth.state_ttl.
//
// env/v2 passes the full env-variable name (the Prefix filter only decides
// inclusion, it does not strip), so we trim the prefix here before matching
// against envSections. Unknown sections are dropped by returning "".
func envTransform(envKey string, value string) (string, any) {
	envKey = strings.TrimPrefix(envKey, envPrefix)
	for _, m := range envSections {
		if strings.HasPrefix(envKey, m.envPrefix) {
			rest := strings.TrimPrefix(envKey, m.envPrefix)
			return m.koanfPath + strings.ToLower(rest), value
		}
	}
	return "", nil
}

func defaults() map[string]any {
	return map[string]any{
		"server.addr":             ":8080",
		"server.base_url":         "http://localhost:8080",
		"server.read_timeout":     "10s",
		"server.write_timeout":    "15s",
		"server.shutdown_timeout": "15s",
		"cache.driver":            "memory",
		"auth.state_ttl":          "10m",
		"auth.session_ttl":        "24h",
		"auth.cookie_secure":      false,
		"auth.cookie_samesite":    "lax",
		"auth.cookie_domain":      "",
		"auth.post_login_path":    "/",
		"internal.base_url":       "http://internal-service:8081",
		"internal.http_timeout":   "5s",
		"internal.cache_ttl":      "60s",
		"providers.github.scopes": []string{"read:user", "public_repo"},
		"logging.level":           "info",
		"logging.format":          "json",
	}
}

// Validate enforces the few invariants we can check at startup.
func (c *Config) Validate() error {
	if c.Server.BaseURL == "" {
		return errors.New("config: server.base_url required")
	}
	if c.Internal.APIKey == "" {
		return errors.New("config: internal.api_key required (set OAUTH_MS_INTERNAL_API_KEY)")
	}
	if c.Providers.GitHub.ClientID == "" || c.Providers.GitHub.ClientSecret == "" {
		return errors.New("config: providers.github.client_id and providers.github.client_secret required")
	}
	return nil
}
