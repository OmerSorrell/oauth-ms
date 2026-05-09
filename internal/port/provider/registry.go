package provider

// Registry resolves a Provider by its URL-segment name.
type Registry interface {
	Get(name string) (Provider, error)
	Names() []string
}
