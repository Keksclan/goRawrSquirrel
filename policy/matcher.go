package policy

import "strings"

// match reports whether r matches fullMethod and, when applicable, returns the
// length of the matched portion (used for tie-breaking among same-kind rules).
func (r *rule) match(fullMethod string) (matched bool, length int) {
	switch r.kind {
	case kindExact:
		if fullMethod == r.pattern {
			return true, len(r.pattern)
		}
	case kindPrefix:
		if strings.HasPrefix(fullMethod, r.pattern) {
			return true, len(r.pattern)
		}
	case kindRegex:
		if loc := r.re.FindStringIndex(fullMethod); loc != nil {
			return true, loc[1] - loc[0]
		}
	}
	return false, 0
}
