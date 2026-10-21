package webui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsText checks which prefixes of a file are taken for text
func TestIsText(t *testing.T) {
	// full is a buffer of sniffLen bytes ending with the first n bytes of tail
	full := func(tail string, n int) []byte {
		return append(bytes.Repeat([]byte("a"), sniffLen-n), tail[:n]...)
	}
	for _, test := range []struct {
		name string
		in   []byte
		want bool
	}{
		{"Empty", nil, true},
		{"ASCII", []byte("hello\nworld"), true},
		{"Utf8", []byte("caffè"), true},
		{"Nul", []byte("a\x00b"), false},
		{"InvalidByte", []byte{'a', 0xff, 'b'}, false},
		{"InvalidAtEnd", []byte{'a', 0xff}, false},
		{"TruncatedShortBuffer", []byte("a\xc3"), false},
		{"FullTruncated2", full("è", 1), true},
		{"FullTruncated3", full("€", 1), true},
		{"FullTruncated3Two", full("€", 2), true},
		{"FullTruncated4Three", full("😀", 3), true},
		{"FullComplete", full("è", 2), true},
		{"FullInvalidAtEnd", full("\xff", 1), false},
		{"FullInvalidInside", append([]byte{0xff}, full("a", 1)[1:]...), false},
		{"FullContinuationsOnly", append(bytes.Repeat([]byte("a"), sniffLen-4), 0x80, 0x80, 0x80, 0x80), false},
		{"FullNul", append(bytes.Repeat([]byte("a"), sniffLen-1), 0), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isText(test.in))
		})
	}
	assert.Len(t, full("è", 1), sniffLen)
}
