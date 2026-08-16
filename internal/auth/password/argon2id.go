package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	defaultMemory      uint32 = 64 * 1024
	defaultIterations  uint32 = 3
	defaultParallelism uint8  = 4
	defaultSaltLength         = 16
	defaultKeyLength   uint32 = 32
)

var (
	ErrInvalidHash          = errors.New("invalid password hash")
	ErrUnsupportedVersion   = errors.New("unsupported argon2 version")
	ErrUnsupportedAlgorithm = errors.New("unsupported password algorithm")
)

type Argon2id struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func NewArgon2id() *Argon2id {
	return &Argon2id{
		memory:      defaultMemory,
		iterations:  defaultIterations,
		parallelism: defaultParallelism,
		saltLength:  defaultSaltLength,
		keyLength:   defaultKeyLength,
	}
}

func (a *Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, a.saltLength)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		a.iterations,
		a.memory,
		a.parallelism,
		a.keyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		a.memory,
		a.iterations,
		a.parallelism,
		encodedSalt,
		encodedHash,
	)

	return encoded, nil
}

func (a *Argon2id) Compare(
	password string,
	encodedHash string,
) (bool, error) {
	params, salt, expectedHash, err := parseEncodedHash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(
		actualHash,
		expectedHash,
	) == 1, nil
}

type parameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parseEncodedHash(
	encodedHash string,
) (parameters, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 {
		return parameters{}, nil, nil, ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return parameters{}, nil, nil, ErrUnsupportedAlgorithm
	}

	version, err := parseVersion(parts[2])
	if err != nil {
		return parameters{}, nil, nil, err
	}

	if version != argon2.Version {
		return parameters{}, nil, nil, ErrUnsupportedVersion
	}

	params, err := parseParameters(parts[3])
	if err != nil {
		return parameters{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return parameters{}, nil, nil, fmt.Errorf(
			"%w: decode salt",
			ErrInvalidHash,
		)
	}

	if len(salt) == 0 {
		return parameters{}, nil, nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return parameters{}, nil, nil, fmt.Errorf(
			"%w: decode hash",
			ErrInvalidHash,
		)
	}

	if len(hash) == 0 {
		return parameters{}, nil, nil, ErrInvalidHash
	}

	return params, salt, hash, nil
}

func parseVersion(value string) (int, error) {
	const prefix = "v="

	if !strings.HasPrefix(value, prefix) {
		return 0, ErrInvalidHash
	}

	version, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	if err != nil {
		return 0, fmt.Errorf(
			"%w: parse version",
			ErrInvalidHash,
		)
	}

	return version, nil
}

func parseParameters(value string) (parameters, error) {
	var params parameters

	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return parameters{}, ErrInvalidHash
	}

	memory, err := parseUint32Parameter(parts[0], "m=")
	if err != nil {
		return parameters{}, err
	}

	iterations, err := parseUint32Parameter(parts[1], "t=")
	if err != nil {
		return parameters{}, err
	}

	parallelism, err := parseUint8Parameter(parts[2], "p=")
	if err != nil {
		return parameters{}, err
	}

	if memory == 0 || iterations == 0 || parallelism == 0 {
		return parameters{}, ErrInvalidHash
	}

	params.memory = memory
	params.iterations = iterations
	params.parallelism = parallelism

	return params, nil
}

func parseUint32Parameter(
	value string,
	prefix string,
) (uint32, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, ErrInvalidHash
	}

	parsed, err := strconv.ParseUint(
		strings.TrimPrefix(value, prefix),
		10,
		32,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: parse %s parameter",
			ErrInvalidHash,
			prefix,
		)
	}

	return uint32(parsed), nil
}

func parseUint8Parameter(
	value string,
	prefix string,
) (uint8, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, ErrInvalidHash
	}

	parsed, err := strconv.ParseUint(
		strings.TrimPrefix(value, prefix),
		10,
		8,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: parse %s parameter",
			ErrInvalidHash,
			prefix,
		)
	}

	return uint8(parsed), nil
}
