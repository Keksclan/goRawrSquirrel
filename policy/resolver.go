package policy

// Resolver holds a set of method groups and resolves a full gRPC method name
// to the best-matching group and its associated policy.
type Resolver struct {
	groups []*GroupBuilder
}

// NewResolver creates a Resolver from the supplied group builders.
func NewResolver(groups ...*GroupBuilder) *Resolver {
	return &Resolver{groups: groups}
}

// Resolve finds the best-matching group for fullMethod.
//
// Priority rules:
//   - Exact matches beat prefix matches, which beat regex matches.
//   - Among matches of the same kind the longer match wins.
//   - When two matches have equal kind and length the group that was
//     registered first (stable order) wins.
//
// If no group matches, ok is false.
func (res *Resolver) Resolve(fullMethod string) (groupName string, pol *Policy, ok bool) {
	bestKind := matchKind(-1)
	bestLen := -1

	for _, g := range res.groups {
		for _, r := range g.rules {
			matched, mLen := r.match(fullMethod)
			if !matched {
				continue
			}
			// A lower kind value means higher priority.
			better := bestKind < 0 ||
				r.kind < bestKind ||
				(r.kind == bestKind && mLen > bestLen)
			if better {
				bestKind = r.kind
				bestLen = mLen
				groupName = g.name
				pol = g.policy
				ok = true
			}
		}
	}
	return groupName, pol, ok
}
