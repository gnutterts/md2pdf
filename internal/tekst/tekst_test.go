package tekst

import (
	"bytes"
	"testing"
)

func TestNaarCP1252BehoudtOndersteundeTekens(t *testing.T) {
	wil := []byte{0xe9, ' ', 0xeb, ' ', 0xef, ' ', 0xf3, ' ', 0xe1, ' ', 0xb7, ' ', 0x97, ' ', 0x85, ' ', 0x9b, ' ', 0xa9, ' ', 0xd7}
	if kreeg := []byte(NaarCP1252("é ë ï ó á · — … › © ×")); !bytes.Equal(kreeg, wil) {
		t.Fatalf("kreeg % x, wil % x", kreeg, wil)
	}
}

func TestNaarCP1252CodeertEnkeleBytes(t *testing.T) {
	wil := []byte{0x63, 0x61, 0x66, 0xe9, 0x20, 0x97, 0x20, 0x9b}
	if kreeg := []byte(NaarCP1252("café — ›")); !bytes.Equal(kreeg, wil) {
		t.Fatalf("kreeg % x, wil % x", kreeg, wil)
	}
}

func TestNaarCP1252VervangtVasteTekens(t *testing.T) {
	if kreeg := NaarCP1252("→←↑↓≥≤≠✓✗\u00a0"); kreeg != "-><-^v>=<=!=vx " {
		t.Fatalf("kreeg %q", kreeg)
	}
}

func TestNaarCP1252VervangtOnbekendTeken(t *testing.T) {
	if kreeg := NaarCP1252("goed 😀"); kreeg != "goed ?" {
		t.Fatalf("kreeg %q", kreeg)
	}
}
