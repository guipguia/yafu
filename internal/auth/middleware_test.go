package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"anonymous", ModeAnonymous, false},
		{"header", ModeHeader, false},
		{"oidc", ModeOIDC, false},
		{"", "", true},
		{"basic", "", true},
	}
	for _, c := range cases {
		got, err := ParseMode(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseMode(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNew_OIDCNotImplemented(t *testing.T) {
	mw, err := New(ModeOIDC)
	if err == nil {
		t.Fatal("expected error for unimplemented oidc mode")
	}
	if mw != nil {
		t.Error("expected nil middleware on error")
	}
}

func TestAnonymous_AlwaysSetsIdentity(t *testing.T) {
	mw, err := New(ModeAnonymous)
	if err != nil {
		t.Fatal(err)
	}
	var captured Identity
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, _ = IdentityFrom(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !captured.IsAnonymous() {
		t.Errorf("expected anonymous identity, got %+v", captured)
	}
	if !captured.HasGroup("system:anonymous") {
		t.Errorf("expected system:anonymous group, got %+v", captured.Groups)
	}
}

func TestHeader_RejectsMissingUser(t *testing.T) {
	mw, err := New(ModeHeader)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if called {
		t.Error("downstream handler should not run on auth failure")
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] == "" {
		t.Errorf("expected error message in body, got %+v", body)
	}
}

func TestHeader_ParsesIdentity(t *testing.T) {
	mw, err := New(ModeHeader)
	if err != nil {
		t.Fatal(err)
	}
	var captured Identity
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, _ = IdentityFrom(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("X-Forwarded-User", "uid-42")
	req.Header.Set("X-Forwarded-Email", "maria@acme.example")
	req.Header.Set("X-Forwarded-Preferred-Username", "maria.k")
	req.Header.Set("X-Forwarded-Groups", "platform-admins, oncall, sre")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	want := Identity{
		Subject: "uid-42",
		Email:   "maria@acme.example",
		Name:    "maria.k",
		Groups:  []string{"platform-admins", "oncall", "sre"},
	}
	if !reflect.DeepEqual(captured, want) {
		t.Errorf("identity = %+v, want %+v", captured, want)
	}
	if !captured.HasGroup("ONCALL") { // case-insensitive
		t.Error("expected case-insensitive HasGroup match")
	}
}

func TestHeader_DefaultsNameToSubject(t *testing.T) {
	mw, _ := New(ModeHeader)
	var captured Identity
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, _ = IdentityFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("X-Forwarded-User", "bare-uid")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if captured.Name != "bare-uid" {
		t.Errorf("Name = %q, want %q (defaulted to Subject)", captured.Name, captured.Subject)
	}
}

func TestSplitGroups(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
	}
	for _, c := range cases {
		got := splitGroups(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitGroups(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestIdentityFrom_MissingReturnsFalse(t *testing.T) {
	if _, ok := IdentityFrom(context.Background()); ok {
		t.Error("expected ok=false for empty context")
	}
}
