package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

type timeSource interface{ Now() time.Time }

// UUID is a canonical RFC 9562 UUIDv7 identifier.
type UUID [16]byte
type UUIDGenerator struct {
	clock  timeSource
	random io.Reader
}

func NewUUIDGenerator(clock timeSource, random io.Reader) (*UUIDGenerator, error) {
	if clock == nil {
		return nil, fmt.Errorf("uuid: clock is required")
	}
	if random == nil {
		random = rand.Reader
	}
	return &UUIDGenerator{clock: clock, random: random}, nil
}
func (generator *UUIDGenerator) New() (UUID, error) {
	var id UUID
	milliseconds := generator.clock.Now().UTC().UnixMilli()
	if milliseconds < 0 || milliseconds > 1<<48-1 {
		return UUID{}, fmt.Errorf("uuid: time out of range")
	}
	if _, err := io.ReadFull(generator.random, id[6:]); err != nil {
		return UUID{}, fmt.Errorf("uuid: random source: %w", err)
	}
	for index := 5; index >= 0; index-- {
		id[index] = byte(milliseconds)
		milliseconds >>= 8
	}
	id[6] = id[6]&0x0f | 0x70
	id[8] = id[8]&0x3f | 0x80
	return id, nil
}
func ParseUUID(value string) (UUID, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return UUID{}, fmt.Errorf("uuid: invalid canonical form")
	}
	compact := value[:8] + value[9:13] + value[14:18] + value[19:23] + value[24:]
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return UUID{}, fmt.Errorf("uuid: invalid hexadecimal")
	}
	var id UUID
	copy(id[:], decoded)
	if id[6]>>4 != 7 || id[8]>>6 != 2 {
		return UUID{}, fmt.Errorf("uuid: UUIDv7 required")
	}
	if id.String() != value {
		return UUID{}, fmt.Errorf("uuid: non-canonical form")
	}
	return id, nil
}
func (id UUID) String() string {
	var output [36]byte
	hex.Encode(output[0:8], id[0:4])
	output[8] = '-'
	hex.Encode(output[9:13], id[4:6])
	output[13] = '-'
	hex.Encode(output[14:18], id[6:8])
	output[18] = '-'
	hex.Encode(output[19:23], id[8:10])
	output[23] = '-'
	hex.Encode(output[24:36], id[10:16])
	return string(output[:])
}
func (id UUID) MarshalText() ([]byte, error) {
	if id[6]>>4 != 7 || id[8]>>6 != 2 {
		return nil, fmt.Errorf("uuid: uninitialized or invalid UUIDv7")
	}
	return []byte(id.String()), nil
}
func (id *UUID) UnmarshalText(text []byte) error {
	parsed, err := ParseUUID(string(text))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
