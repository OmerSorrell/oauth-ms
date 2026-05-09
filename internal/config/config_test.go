package config

import "testing"

func TestEnvTransform(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"OAUTH_MS_SERVER_ADDR", "server.addr"},
		{"OAUTH_MS_SERVER_BASE_URL", "server.base_url"},
		{"OAUTH_MS_AUTH_STATE_TTL", "auth.state_ttl"},
		{"OAUTH_MS_AUTH_COOKIE_SAMESITE", "auth.cookie_samesite"},
		{"OAUTH_MS_INTERNAL_API_KEY", "internal.api_key"},
		{"OAUTH_MS_INTERNAL_HTTP_TIMEOUT", "internal.http_timeout"},
		{"OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID", "providers.github.client_id"},
		{"OAUTH_MS_PROVIDERS_GITHUB_CLIENT_SECRET", "providers.github.client_secret"},
		{"OAUTH_MS_LOGGING_LEVEL", "logging.level"},
		{"OAUTH_MS_CACHE_DRIVER", "cache.driver"},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			got, _ := envTransform(tc.env, "v")
			if got != tc.want {
				t.Fatalf("envTransform(%s) = %q, want %q", tc.env, got, tc.want)
			}
		})
	}
}

func TestEnvTransform_UnknownSectionDropped(t *testing.T) {
	if got, _ := envTransform("OAUTH_MS_UNKNOWN_KEY", "v"); got != "" {
		t.Fatalf("expected empty key for unknown section, got %q", got)
	}
}
