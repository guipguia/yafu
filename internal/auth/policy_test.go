package auth

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	maria = Identity{Subject: "uid-1", Email: "maria@acme.example", Groups: []string{"platform-admins", "sre"}}
	dev   = Identity{Subject: "uid-2", Email: "dev@acme.example", Groups: []string{"dev-team"}}
	noone = Identity{}
)

func TestAuthorize_DefaultAllow(t *testing.T) {
	if !DefaultAllowAllPolicy.Authorize(maria, "get", "any") {
		t.Error("default allow-all should allow")
	}
	if !DefaultAllowAllPolicy.Authorize(noone, "anything", "anywhere") {
		t.Error("default allow-all should allow even empty identity")
	}
}

func TestAuthorize_DefaultDeny(t *testing.T) {
	p := Policy{DefaultAction: ActionDeny}
	if p.Authorize(maria, "get", "anywhere") {
		t.Error("default deny should reject")
	}
}

func TestAuthorize_FirstMatchWins(t *testing.T) {
	p := Policy{
		DefaultAction: ActionDeny,
		Rules: []Rule{
			// Earlier rule denies; later rule would allow — earlier wins.
			{Subjects: []string{"group:sre"}, Verbs: []string{"get"}, Clusters: []string{"prod-*"}, Action: ActionDeny},
			{Subjects: []string{"*"}, Verbs: []string{"*"}, Clusters: []string{"*"}, Action: ActionAllow},
		},
	}
	if p.Authorize(maria, "get", "prod-eu-west") {
		t.Error("expected deny via earlier rule")
	}
	if !p.Authorize(maria, "get", "edge-tokyo") {
		t.Error("expected allow via later catch-all rule")
	}
}

func TestAuthorize_GroupMatch(t *testing.T) {
	p := Policy{
		DefaultAction: ActionDeny,
		Rules: []Rule{
			{Subjects: []string{"group:platform-admins"}, Verbs: []string{"*"}, Clusters: []string{"*"}, Action: ActionAllow},
		},
	}
	if !p.Authorize(maria, "reconcile", "any") {
		t.Error("maria has platform-admins → should allow")
	}
	if p.Authorize(dev, "get", "any") {
		t.Error("dev not in platform-admins → should deny")
	}
}

func TestAuthorize_UserMatch(t *testing.T) {
	p := Policy{
		DefaultAction: ActionDeny,
		Rules: []Rule{
			{Subjects: []string{"user:maria@acme.example"}, Verbs: []string{"get"}, Clusters: []string{"*"}, Action: ActionAllow},
			{Subjects: []string{"user:uid-2"}, Verbs: []string{"get"}, Clusters: []string{"*"}, Action: ActionAllow},
		},
	}
	if !p.Authorize(maria, "get", "any") {
		t.Error("maria by email → allow")
	}
	if !p.Authorize(dev, "get", "any") {
		t.Error("dev by subject id → allow")
	}
	if p.Authorize(maria, "reconcile", "any") {
		t.Error("verb 'reconcile' not listed → deny")
	}
}

func TestAuthorize_ClusterGlob(t *testing.T) {
	p := Policy{
		DefaultAction: ActionDeny,
		Rules: []Rule{
			{Subjects: []string{"group:dev-team"}, Verbs: []string{"get"}, Clusters: []string{"dev-*", "staging"}, Action: ActionAllow},
		},
	}
	if !p.Authorize(dev, "get", "dev-001") {
		t.Error("dev-001 matches glob dev-*")
	}
	if !p.Authorize(dev, "get", "staging") {
		t.Error("exact 'staging' match")
	}
	if p.Authorize(dev, "get", "prod-eu-west") {
		t.Error("prod-eu-west should NOT match dev-* / staging")
	}
}

func TestAuthorize_VerbWildcard(t *testing.T) {
	p := Policy{
		Rules: []Rule{
			{Subjects: []string{"group:platform-admins"}, Verbs: []string{"*"}, Clusters: []string{"*"}, Action: ActionAllow},
		},
		DefaultAction: ActionDeny,
	}
	for _, verb := range []string{"get", "list", "reconcile", "suspend", "resume", "anything"} {
		if !p.Authorize(maria, verb, "any") {
			t.Errorf("verb %q: expected allow", verb)
		}
	}
}

func TestAuthorize_StarSubject(t *testing.T) {
	p := Policy{
		Rules: []Rule{
			{Subjects: []string{"*"}, Verbs: []string{"get"}, Clusters: []string{"public"}, Action: ActionAllow},
		},
		DefaultAction: ActionDeny,
	}
	if !p.Authorize(noone, "get", "public") {
		t.Error("* subject matches everyone — including empty identity")
	}
	if p.Authorize(noone, "reconcile", "public") {
		t.Error("verb mismatch — should deny")
	}
}

func TestLoadPolicyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	body := `defaultAction: deny
rules:
  - subjects: ["group:platform-admins"]
    verbs: ["*"]
    clusters: ["*"]
    action: allow
  - subjects: ["group:dev-team"]
    verbs: ["get"]
    clusters: ["dev-*"]
    action: allow
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicyFile(path)
	if err != nil {
		t.Fatalf("LoadPolicyFile: %v", err)
	}
	if p.DefaultAction != ActionDeny {
		t.Errorf("defaultAction = %q, want deny", p.DefaultAction)
	}
	if len(p.Rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(p.Rules))
	}

	if !p.Authorize(maria, "reconcile", "edge-tokyo") {
		t.Error("admin rule should allow")
	}
	if !p.Authorize(dev, "get", "dev-001") {
		t.Error("dev-team get on dev-* should allow")
	}
	if p.Authorize(dev, "get", "prod-eu-west") {
		t.Error("dev-team get on prod-eu-west should deny via default")
	}
}

func TestLoadPolicyFile_DefaultsDenyWhenUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte("rules: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPolicyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultAction != ActionDeny {
		t.Errorf("expected DefaultAction=deny when unset in file, got %q", p.DefaultAction)
	}
}

func TestLoadPolicyFile_Missing(t *testing.T) {
	if _, err := LoadPolicyFile("/nonexistent/policy.yaml"); err == nil {
		t.Error("expected error for missing file")
	}
}
