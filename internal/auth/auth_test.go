package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantErr   error
		checkWith string // password to verify against the resulting hash; "" = use original
		wantMatch bool
	}{
		{name: "valid password round-trips", password: "supersecret1", checkWith: "", wantMatch: true},
		{name: "too short password rejected", password: "short", wantErr: ErrPasswordTooShort},
		{name: "wrong password fails check", password: "supersecret1", checkWith: "wrongpassword", wantMatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error hashing: %v", err)
			}

			checkPwd := tt.checkWith
			if checkPwd == "" {
				checkPwd = tt.password
			}
			err = CheckPassword(hash, checkPwd)
			matched := err == nil
			if matched != tt.wantMatch {
				t.Fatalf("expected match=%v, got match=%v (err=%v)", tt.wantMatch, matched, err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"empty email", "", true},
		{"missing at", "userexample.com", true},
		{"at at start", "@example.com", true},
		{"at at end", "user@", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateEmail(%q): wantErr=%v, got err=%v", tt.email, tt.wantErr, err)
			}
		})
	}
}

func TestIssueAndVerifyToken(t *testing.T) {
	tests := []struct {
		name      string
		ttl       time.Duration
		wait      time.Duration
		wantValid bool
	}{
		{name: "fresh token is valid", ttl: time.Hour, wait: 0, wantValid: true},
		{name: "expired token is rejected", ttl: 1 * time.Millisecond, wait: 10 * time.Millisecond, wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuer := NewTokenIssuer("test-secret", tt.ttl)
			token, err := issuer.IssueToken("user-123", "user@example.com")
			if err != nil {
				t.Fatalf("unexpected error issuing token: %v", err)
			}
			time.Sleep(tt.wait)

			claims, err := issuer.VerifyToken(token)
			valid := err == nil
			if valid != tt.wantValid {
				t.Fatalf("expected valid=%v, got valid=%v (err=%v)", tt.wantValid, valid, err)
			}
			if valid {
				if claims.UserID != "user-123" || claims.Email != "user@example.com" {
					t.Fatalf("unexpected claims: %+v", claims)
				}
			}
		})
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	issuer := NewTokenIssuer("secret-a", time.Hour)
	token, err := issuer.IssueToken("user-1", "a@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	other := NewTokenIssuer("secret-b", time.Hour)
	if _, err := other.VerifyToken(token); err == nil {
		t.Fatal("expected error verifying token signed with a different secret")
	}
}

func TestVerifyToken_Malformed(t *testing.T) {
	issuer := NewTokenIssuer("secret", time.Hour)
	if _, err := issuer.VerifyToken("not.a.jwt"); err == nil {
		t.Fatal("expected error for malformed token")
	}
	if _, err := issuer.VerifyToken(""); err == nil {
		t.Fatal("expected error for empty token")
	}
	// sanity check error wrapping message
	_, err := issuer.VerifyToken("garbage")
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}