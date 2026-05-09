// Package profile holds the connector's canonical, provider-agnostic profile types.
package profile

// NormalizedProfile is the /profile response shape, independent of which provider authenticated the user.
type NormalizedProfile struct {
	User      User       `json:"user"`
	Resources []Resource `json:"resources"`
}
