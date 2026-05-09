// Package registry is the default Registry implementation: a name → Provider map
// frozen at startup.
package registry

import (
	"fmt"
	"sort"

	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
)

// Registry is an immutable map of provider name → Provider.
type Registry struct {
	providers map[string]provider.Provider
}

// New validates and freezes the given providers. Returns error on nil entries,
// empty names, or duplicates.
func New(provs ...provider.Provider) (*Registry, error) {
	m := make(map[string]provider.Provider, len(provs))
	for _, p := range provs {
		if p == nil {
			return nil, fmt.Errorf("registry: nil provider")
		}
		name := p.Name()
		if name == "" {
			return nil, fmt.Errorf("registry: provider with empty name")
		}
		if _, exists := m[name]; exists {
			return nil, fmt.Errorf("registry: duplicate provider %q", name)
		}
		m[name] = p
	}
	return &Registry{providers: m}, nil
}

func (r *Registry) Get(name string) (provider.Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", provider.ErrUnknownProvider, name)
	}
	return p, nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

var _ provider.Registry = (*Registry)(nil)
