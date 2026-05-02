package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Action is what a Rule does when it matches.
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

// Rule is one entry in a Policy. A request is matched by a rule when the
// identity's subject/email or one of its groups appears in Subjects, the
// requested verb appears in Verbs (or "*"), and the cluster id matches one
// of Clusters by exact match or filepath-glob (e.g. "prod-*").
//
// Subject syntax:
//   - "user:<email-or-subject>"  — match by Identity.Email or Identity.Subject
//   - "group:<name>"             — case-insensitive group membership
//   - "*"                        — match everyone
type Rule struct {
	Subjects []string `json:"subjects,omitempty"`
	Verbs    []string `json:"verbs,omitempty"`
	Clusters []string `json:"clusters,omitempty"`

	// Action defaults to allow when empty.
	Action Action `json:"action,omitempty"`
}

// Policy is a list of Rules evaluated in order; the first matching rule's
// Action wins. If no rule matches, DefaultAction applies.
type Policy struct {
	// DefaultAction is what happens when no rule matches. Empty defaults
	// to allow for the built-in DefaultAllowAllPolicy and to deny for
	// any policy loaded from disk via LoadPolicyFile.
	DefaultAction Action `json:"defaultAction,omitempty"`
	Rules         []Rule `json:"rules,omitempty"`
}

// DefaultAllowAllPolicy is applied when --rbac-file is not set: every
// authenticated request is allowed. main.go logs a WARN so this is
// obvious at startup.
var DefaultAllowAllPolicy = Policy{DefaultAction: ActionAllow}

// Authorize returns true when id may perform verb on cluster.
//
// The identity may be the zero Identity{} when called from contexts where
// no auth ran (test setup, internal probes); rules may still match it via
// "*" subjects.
func (p Policy) Authorize(id Identity, verb, cluster string) bool {
	for _, r := range p.Rules {
		if r.matches(id, verb, cluster) {
			return r.Action != ActionDeny // empty Action treated as allow
		}
	}
	return p.DefaultAction != ActionDeny
}

func (r Rule) matches(id Identity, verb, cluster string) bool {
	return r.matchSubjects(id) && matchAny(r.Verbs, verb) && matchClusters(r.Clusters, cluster)
}

func (r Rule) matchSubjects(id Identity) bool {
	for _, s := range r.Subjects {
		if s == "*" {
			return true
		}
		if u, ok := strings.CutPrefix(s, "user:"); ok {
			if u != "" && (u == id.Subject || u == id.Email) {
				return true
			}
			continue
		}
		if g, ok := strings.CutPrefix(s, "group:"); ok {
			if g != "" && id.HasGroup(g) {
				return true
			}
		}
	}
	return false
}

// matchAny returns true when target equals "*" or one of list (or list
// contains "*"). Used for verbs.
func matchAny(list []string, target string) bool {
	for _, v := range list {
		if v == "*" || v == target {
			return true
		}
	}
	return false
}

// matchClusters returns true when target matches any pattern in list,
// supporting filepath-glob ("prod-*", "edge-tokyo", "*").
func matchClusters(list []string, target string) bool {
	for _, p := range list {
		if p == "*" || p == target {
			return true
		}
		if ok, _ := filepath.Match(p, target); ok {
			return true
		}
	}
	return false
}

// LoadPolicyFile reads a YAML/JSON policy file from disk. An unset
// DefaultAction is interpreted as deny — explicit policies should be
// safe-by-default.
func LoadPolicyFile(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy %s: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("parse policy %s: %w", path, err)
	}
	if p.DefaultAction == "" {
		p.DefaultAction = ActionDeny
	}
	return p, nil
}
