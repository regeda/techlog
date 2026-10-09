package event

import (
	"fmt"
	"io"
	"time"
)

const idAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NewID returns "evt_" followed by 12 characters from [0-9A-Z]: eight base-36
// characters of t in Unix milliseconds (so ids sort by creation time) and four
// random base-36 characters read from r.
func NewID(t time.Time, r io.Reader) (string, error) {
	var b [12]byte
	ms := uint64(t.UnixMilli())
	for i := 7; i >= 0; i-- {
		b[i] = idAlphabet[ms%36]
		ms /= 36
	}
	var rnd [4]byte
	if _, err := io.ReadFull(r, rnd[:]); err != nil {
		return "", fmt.Errorf("read id entropy: %w", err)
	}
	for i, v := range rnd {
		b[8+i] = idAlphabet[int(v)%36]
	}
	return "evt_" + string(b[:]), nil
}
