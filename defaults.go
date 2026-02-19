package gorawrsquirrel

// DefaultOptions returns the recommended set of options for production use.
// Currently this includes panic recovery; additional defaults may be added
// in future versions.
func DefaultOptions() []Option {
	return []Option{
		WithRecovery(),
	}
}
