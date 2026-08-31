package shared

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const MaxCursorBytes = 4096

type CursorScope struct {
	TenantID    UUID
	Endpoint    string
	QueryDigest Digest
	PageSize    int
}
type Cursor struct {
	Position  string
	ExpiresAt time.Time
}
type cursorPayload struct {
	Version  int    `json:"v"`
	Tenant   string `json:"t"`
	Endpoint string `json:"e"`
	Query    string `json:"q"`
	PageSize int    `json:"s"`
	Position string `json:"p"`
	Expires  int64  `json:"x"`
}
type CursorCodec struct {
	key   []byte
	clock timeSource
}

func NewCursorCodec(key []byte, clock timeSource) (*CursorCodec, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("cursor: key must contain at least 32 bytes")
	}
	if clock == nil {
		return nil, fmt.Errorf("cursor: clock is required")
	}
	return &CursorCodec{key: append([]byte(nil), key...), clock: clock}, nil
}
func validateCursorScope(scope CursorScope) error {
	if _, err := scope.TenantID.MarshalText(); err != nil {
		return fmt.Errorf("cursor: tenant: %w", err)
	}
	if scope.Endpoint == "" || len(scope.Endpoint) > 256 || scope.Endpoint[0] != '/' || strings.ContainsAny(scope.Endpoint, "?#\r\n") {
		return fmt.Errorf("cursor: invalid endpoint")
	}
	if scope.QueryDigest.Algorithm() != SHA256 {
		return fmt.Errorf("cursor: query digest required")
	}
	if scope.PageSize < 1 || scope.PageSize > 200 {
		return fmt.Errorf("cursor: invalid page size")
	}
	return nil
}
func (codec *CursorCodec) Encode(scope CursorScope, cursor Cursor) (string, error) {
	if err := validateCursorScope(scope); err != nil {
		return "", err
	}
	if _, err := NewBoundedString(cursor.Position, 1024); err != nil || !cursor.ExpiresAt.After(codec.clock.Now()) {
		return "", fmt.Errorf("cursor: invalid position or expiry")
	}
	payload := cursorPayload{1, scope.TenantID.String(), scope.Endpoint, codec.queryBinding(scope.QueryDigest), scope.PageSize, cursor.Position, cursor.ExpiresAt.UTC().Unix()}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("cursor: encode payload: %w", err)
	}
	result := base64.RawURLEncoding.EncodeToString(encoded) + "." + base64.RawURLEncoding.EncodeToString(codec.sign(encoded))
	if len(result) > MaxCursorBytes {
		return "", fmt.Errorf("cursor: encoded value too large")
	}
	return result, nil
}
func (codec *CursorCodec) Decode(encoded string, expected CursorScope) (Cursor, error) {
	if err := validateCursorScope(expected); err != nil {
		return Cursor{}, err
	}
	if len(encoded) < 1 || len(encoded) > MaxCursorBytes {
		return Cursor{}, fmt.Errorf("cursor: invalid size")
	}
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 {
		return Cursor{}, fmt.Errorf("cursor: invalid form")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Cursor{}, fmt.Errorf("cursor: invalid encoding")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, codec.sign(payloadBytes)) {
		return Cursor{}, fmt.Errorf("cursor: authentication failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(payloadBytes))
	decoder.DisallowUnknownFields()
	var payload cursorPayload
	if err := decoder.Decode(&payload); err != nil {
		return Cursor{}, fmt.Errorf("cursor: invalid payload")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Cursor{}, fmt.Errorf("cursor: invalid payload")
	}
	if payload.Version != 1 || payload.Tenant != expected.TenantID.String() || payload.Endpoint != expected.Endpoint || payload.Query != codec.queryBinding(expected.QueryDigest) || payload.PageSize != expected.PageSize {
		return Cursor{}, fmt.Errorf("cursor: scope mismatch")
	}
	expires := time.Unix(payload.Expires, 0).UTC()
	if _, err := NewBoundedString(payload.Position, 1024); err != nil || !expires.After(codec.clock.Now()) {
		return Cursor{}, fmt.Errorf("cursor: expired or invalid")
	}
	return Cursor{Position: payload.Position, ExpiresAt: expires}, nil
}
func (codec *CursorCodec) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, codec.key)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func (codec *CursorCodec) queryBinding(digest Digest) string {
	mac := hmac.New(sha256.New, codec.key)
	_, _ = mac.Write([]byte("thinkpixelmp.cursor.query.v1\x00"))
	_, _ = mac.Write([]byte(digest.String()))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
