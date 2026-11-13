package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameters are configurable as stored hashes carry their own parameters.
// The initial values use 64 MiB and 3 passes; benchmark on deployment hardware
// and apply a rate limit to login attempts before production use.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", fmt.Errorf("password must be at least 12 bytes")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, encode(salt), encode(hash)), nil
}

func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, fmt.Errorf("invalid password hash format")
	}
	var memory, iterations uint32
	var threads uint8
	for _, parameter := range strings.Split(parts[3], ",") {
		pair := strings.SplitN(parameter, "=", 2)
		if len(pair) != 2 {
			return false, fmt.Errorf("invalid argon2 parameter")
		}
		value, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			return false, fmt.Errorf("invalid argon2 parameter: %w", err)
		}
		switch pair[0] {
		case "m":
			memory = uint32(value)
		case "t":
			iterations = uint32(value)
		case "p":
			if value > 255 {
				return false, fmt.Errorf("invalid argon2 parallelism")
			}
			threads = uint8(value)
		default:
			return false, fmt.Errorf("unknown argon2 parameter")
		}
	}
	if memory < 8*1024 || memory > 1024*1024 || iterations == 0 || iterations > 10 || threads == 0 {
		return false, fmt.Errorf("argon2 parameters out of permitted range")
	}
	decode := base64.RawStdEncoding.DecodeString
	salt, err := decode(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false, fmt.Errorf("invalid password salt")
	}
	expected, err := decode(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false, fmt.Errorf("invalid password digest")
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(expected, actual) == 1, nil
}
