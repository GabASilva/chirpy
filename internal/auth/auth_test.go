package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "mysecretpassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if hash == "" {
		t.Fatalf("Expected non-empty hash, got empty string")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "mysecretpassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	isMatch, err := CheckPasswordHash(password, hash)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !isMatch {
		t.Fatalf("Expected password to match hash")
	}
}

func TestSamePasswordDifferentHashesBothMatch(t *testing.T) {
	password := "mysecretpassword"
	hash1, err3 := HashPassword(password)
	if err3 != nil {
		t.Fatalf("Failed to hash1 password, got %v", err3)
	}
	hash2, err4 := HashPassword(password)
	if err4 != nil {
		t.Fatalf("Failed to hash2 password, got %v", err4)
	}
	isMatch, err := CheckPasswordHash(password, hash1)
	isMatch2, err2 := CheckPasswordHash(password, hash2)
	if err != nil || err2 != nil {
		t.Fatalf("Expected no error, got %v and %v", err, err2)
	}
	if !isMatch {
		t.Fatalf("Expected password to match hash")
	}
	if !isMatch2 {
		t.Fatalf("Expected password to match hash2")
	}
	if hash1 == hash2 {
		t.Fatalf("Expected different hashes for same password, got same hash")
	}
}

func TestWrongPasswordDoesNotMatchHash(t *testing.T) {
	password := "mysecretpassword"
	wrongPassword := "wrongpassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password, got %v", err)
	}
	isMatch, err := CheckPasswordHash(wrongPassword, hash)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if isMatch {
		t.Fatalf("Expected wrong password to not match hash")
	}
}
