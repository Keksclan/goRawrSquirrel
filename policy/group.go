package policy

import (
	"regexp"
	"time"
)

// RateLimitRule describes a rate-limiting policy for a group of methods.
type RateLimitRule struct {
	// Rate is the maximum number of requests allowed within Window.
	Rate int
	// Window is the time window for the rate limit.
	Window time.Duration
}

// Policy holds the configuration that applies to a matched method group.
type Policy struct {
	RateLimit    *RateLimitRule
	Timeout      time.Duration
	AuthRequired bool
}

// matchKind distinguishes the three matching strategies.
type matchKind int

const (
	kindExact  matchKind = iota // highest priority
	kindPrefix                  // medium priority
	kindRegex                   // lowest priority
)

// rule is a single matching rule inside a group.
type rule struct {
	kind    matchKind
	pattern string         // used for exact and prefix matches
	re      *regexp.Regexp // used for regex matches
}

// GroupBuilder constructs a method group with one or more matching rules and
// a policy.
type GroupBuilder struct {
	name   string
	rules  []rule
	policy *Policy
}

// Group starts building a new method group with the given name.
func Group(name string) *GroupBuilder {
	return &GroupBuilder{name: name}
}

// Exact adds an exact-match rule for pattern.
func (g *GroupBuilder) Exact(pattern string) *GroupBuilder {
	g.rules = append(g.rules, rule{kind: kindExact, pattern: pattern})
	return g
}

// Prefix adds a prefix-match rule for pattern.
func (g *GroupBuilder) Prefix(pattern string) *GroupBuilder {
	g.rules = append(g.rules, rule{kind: kindPrefix, pattern: pattern})
	return g
}

// Regex adds a regex-match rule for pattern.
// The pattern is compiled immediately; an invalid regex will panic.
func (g *GroupBuilder) Regex(pattern string) *GroupBuilder {
	g.rules = append(g.rules, rule{kind: kindRegex, pattern: pattern, re: regexp.MustCompile(pattern)})
	return g
}

// Policy attaches a Policy to the group and returns the finished builder.
func (g *GroupBuilder) Policy(p Policy) *GroupBuilder {
	g.policy = &p
	return g
}
