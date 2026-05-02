package auth

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestStringSliceFromClaim(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, nil},
		{"native string slice", []string{"a", "b"}, []string{"a", "b"}},
		{"interface slice", []any{"a", "b", 3, ""}, []string{"a", "b"}},
		{"single string with commas (Azure-style)", "a, b , c", []string{"a", "b", "c"}},
		{"empty string", "", nil},
		{"unsupported type", 42, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stringSliceFromClaim(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestSignCookieValue_RoundTrip(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32-bytes-long")
	body := []byte(`{"foo":"bar","n":42}`)

	signed := signCookieValue(secret, body)
	if !strings.Contains(signed, ".") {
		t.Fatalf("expected `body.signature` shape, got %q", signed)
	}

	got, err := verifyCookieValue(secret, signed)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("got %q, want %q", got, body)
	}
}

func TestVerifyCookieValue_TamperDetected(t *testing.T) {
	secret := []byte("test-secret-must-be-at-least-32-bytes-long")
	signed := signCookieValue(secret, []byte("hello"))

	// Flip a byte in the body half.
	parts := strings.SplitN(signed, ".", 2)
	parts[0] = "AAAA" + parts[0][4:]
	tampered := parts[0] + "." + parts[1]

	if _, err := verifyCookieValue(secret, tampered); err == nil {
		t.Error("expected tampered body to fail verification")
	}
}

func TestVerifyCookieValue_DifferentSecret(t *testing.T) {
	signed := signCookieValue([]byte("alice"), []byte("hello"))
	if _, err := verifyCookieValue([]byte("bob"), signed); err == nil {
		t.Error("expected verification to fail under a different secret")
	}
}

func TestVerifyCookieValue_MalformedRejected(t *testing.T) {
	if _, err := verifyCookieValue([]byte("k"), "no-dot"); err == nil {
		t.Error("expected error for malformed value")
	}
	if _, err := verifyCookieValue([]byte("k"), "AAAA.!!!"); err == nil {
		t.Error("expected error for non-base64 sig")
	}
}

func TestSanitizeReturnTo(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/apps", "/apps"},
		{"/apps?cluster=prod", "/apps?cluster=prod"},
		{"https://evil.example/foo", ""},        // absolute URL → reject
		{"//evil.example/foo", ""},               // protocol-relative → reject
		{"javascript:alert(1)", ""},              // scheme → reject
		{"apps", ""},                             // no leading / → reject
	}
	for _, c := range cases {
		if got := sanitizeReturnTo(c.in); got != c.want {
			t.Errorf("sanitizeReturnTo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRandomString_Distinct(t *testing.T) {
	a := randomString(24)
	b := randomString(24)
	if a == b {
		t.Errorf("expected randomString to produce different values, got %q twice", a)
	}
	if len(a) < 24 {
		t.Errorf("randomString(24) shorter than expected: %q", a)
	}
}
