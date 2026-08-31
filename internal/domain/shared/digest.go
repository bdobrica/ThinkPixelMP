package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type DigestAlgorithm string

const SHA256 DigestAlgorithm = "sha256"

// Digest is an algorithm-aware immutable content identity.
type Digest struct {
	algorithm DigestAlgorithm
	bytes     [32]byte
}

// SHA256Digest hashes exact bytes into a typed digest. Callers remain
// responsible for canonicalizing structured input before hashing it.
func SHA256Digest(value []byte) Digest {
	return Digest{algorithm: SHA256, bytes: sha256.Sum256(value)}
}

func ParseDigest(value string) (Digest, error) {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return Digest{}, fmt.Errorf("digest: expected canonical sha256")
	}
	hexValue := value[7:]
	if strings.ToLower(hexValue) != hexValue {
		return Digest{}, fmt.Errorf("digest: uppercase hexadecimal is not canonical")
	}
	decoded, err := hex.DecodeString(hexValue)
	if err != nil || len(decoded) != 32 {
		return Digest{}, fmt.Errorf("digest: invalid sha256 value")
	}
	result := Digest{algorithm: SHA256}
	copy(result.bytes[:], decoded)
	return result, nil
}
func (digest Digest) Algorithm() DigestAlgorithm { return digest.algorithm }
func (digest Digest) String() string {
	if digest.algorithm != SHA256 {
		return ""
	}
	return "sha256:" + hex.EncodeToString(digest.bytes[:])
}
func (digest Digest) MarshalText() ([]byte, error) {
	if digest.algorithm != SHA256 {
		return nil, fmt.Errorf("digest: uninitialized")
	}
	return []byte(digest.String()), nil
}
func (digest *Digest) UnmarshalText(text []byte) error {
	parsed, err := ParseDigest(string(text))
	if err != nil {
		return err
	}
	*digest = parsed
	return nil
}
