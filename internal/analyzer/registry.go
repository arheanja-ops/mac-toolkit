package analyzer

// registry holds all registered analyzers
var registry []Analyzer

// Register adds an analyzer to the global registry
func Register(a Analyzer) {
	registry = append(registry, a)
}

// All returns all registered analyzers
func All() []Analyzer {
	return registry
}

// ByDomain returns the analyzer for a specific domain, or nil
func ByDomain(domain string) Analyzer {
	for _, a := range registry {
		if a.Domain() == domain {
			return a
		}
	}
	return nil
}

// Domains returns all registered domain names
func Domains() []string {
	names := make([]string, len(registry))
	for i, a := range registry {
		names[i] = a.Domain()
	}
	return names
}
