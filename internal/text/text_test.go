package text

import (
	"bytes"
	"testing"
)

func TestToCP1252PreservesSupportedCharacters(t *testing.T) {
	want := []byte{0xe9, ' ', 0xeb, ' ', 0xef, ' ', 0xf3, ' ', 0xe1, ' ', 0xb7, ' ', 0x97, ' ', 0x85, ' ', 0x9b, ' ', 0xa9, ' ', 0xd7}
	if got := []byte(ToCP1252("é ë ï ó á · — … › © ×")); !bytes.Equal(got, want) {
		t.Fatalf("received % x, want % x", got, want)
	}
}

func TestToCP1252EncodesSingleBytes(t *testing.T) {
	want := []byte{0x63, 0x61, 0x66, 0xe9, 0x20, 0x97, 0x20, 0x9b}
	if got := []byte(ToCP1252("café — ›")); !bytes.Equal(got, want) {
		t.Fatalf("received % x, want % x", got, want)
	}
}

func TestToCP1252ReplacesFixedCharacters(t *testing.T) {
	if got := ToCP1252("→←↑↓≥≤≠✓✗\u00a0"); got != "-><-^v>=<=!=vx " {
		t.Fatalf("received %q", got)
	}
}

func TestToCP1252ReplacesUnknownCharacter(t *testing.T) {
	if got := ToCP1252("goed 😀"); got != "goed ?" {
		t.Fatalf("received %q", got)
	}
}
