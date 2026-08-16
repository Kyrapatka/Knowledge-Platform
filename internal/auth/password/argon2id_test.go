package password

import (
	"errors"
	"strings"
	"testing"
)

func TestArgon2id_HashAndCompare(t *testing.T) {
	hasher := NewArgon2id()

	const password = "strong-password-123"

	encodedHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if encodedHash == "" {
		t.Fatal("encoded hash must not be empty")
	}

	if strings.Contains(encodedHash, password) {
		t.Fatal("encoded hash must not contain the original password")
	}

	match, err := hasher.Compare(password, encodedHash)
	if err != nil {
		t.Fatalf("compare correct password: %v", err)
	}

	if !match {
		t.Fatal("correct password must match")
	}
}

func TestArgon2id_CompareWrongPassword(t *testing.T) {
	hasher := NewArgon2id()

	encodedHash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	match, err := hasher.Compare(
		"wrong-password",
		encodedHash,
	)
	if err != nil {
		t.Fatalf("compare wrong password: %v", err)
	}

	if match {
		t.Fatal("wrong password must not match")
	}
}

func TestArgon2id_HashUsesRandomSalt(t *testing.T) {
	hasher := NewArgon2id()

	const password = "same-password"

	firstHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("create first hash: %v", err)
	}

	secondHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("create second hash: %v", err)
	}

	if firstHash == secondHash {
		t.Fatal("hashes for the same password must be different")
	}

	firstMatch, err := hasher.Compare(password, firstHash)
	if err != nil {
		t.Fatalf("compare first hash: %v", err)
	}

	if !firstMatch {
		t.Fatal("password must match the first hash")
	}

	secondMatch, err := hasher.Compare(password, secondHash)
	if err != nil {
		t.Fatalf("compare second hash: %v", err)
	}

	if !secondMatch {
		t.Fatal("password must match the second hash")
	}
}

func TestArgon2id_CompareInvalidHash(t *testing.T) {
	hasher := NewArgon2id()

	testCases := []struct {
		name        string
		encodedHash string
	}{
		{
			name:        "empty hash",
			encodedHash: "",
		},
		{
			name:        "random text",
			encodedHash: "not-an-argon2-hash",
		},
		{
			name:        "missing hash section",
			encodedHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA",
		},
		{
			name:        "invalid parameters",
			encodedHash: "$argon2id$v=19$m=wrong,t=3,p=4$c2FsdA$aGFzaA",
		},
		{
			name:        "invalid salt",
			encodedHash: "$argon2id$v=19$m=65536,t=3,p=4$***$aGFzaA",
		},
		{
			name:        "invalid hash",
			encodedHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$***",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			match, err := hasher.Compare(
				"password",
				testCase.encodedHash,
			)

			if match {
				t.Fatal("invalid hash must not match")
			}

			if !errors.Is(err, ErrInvalidHash) {
				t.Fatalf(
					"expected ErrInvalidHash, got %v",
					err,
				)
			}
		})
	}
}

func TestArgon2id_CompareUnsupportedAlgorithm(t *testing.T) {
	hasher := NewArgon2id()

	encodedHash := "$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"

	match, err := hasher.Compare(
		"password",
		encodedHash,
	)

	if match {
		t.Fatal("unsupported algorithm must not match")
	}

	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf(
			"expected ErrUnsupportedAlgorithm, got %v",
			err,
		)
	}
}

func TestArgon2id_CompareUnsupportedVersion(t *testing.T) {
	hasher := NewArgon2id()

	encodedHash := "$argon2id$v=18$m=65536,t=3,p=4$c2FsdA$aGFzaA"

	match, err := hasher.Compare(
		"password",
		encodedHash,
	)

	if match {
		t.Fatal("unsupported version must not match")
	}

	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf(
			"expected ErrUnsupportedVersion, got %v",
			err,
		)
	}
}
