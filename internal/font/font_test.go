// SPDX-License-Identifier: MIT

package font

import (
	"bytes"
	"testing"
)

func TestEveryStyleIsEmbedded(t *testing.T) {
	for _, family := range []string{Sans, Mono} {
		for _, style := range []string{"", "B", "I", "BI"} {
			data, err := Bytes(family, style)
			// A TrueType file starts with the version 1.0 tag 00 01 00 00.
			if err != nil || len(data) < 100_000 || !bytes.HasPrefix(data, []byte{0, 1, 0, 0}) {
				t.Errorf("%s %q: %d bytes, err = %v", family, style, len(data), err)
			}
		}
	}
	if _, err := Bytes("Serif", ""); err == nil {
		t.Error("an unknown family gave no error")
	}
	if _, err := Bytes(Sans, "U"); err == nil {
		t.Error("an unknown style gave no error")
	}
}
