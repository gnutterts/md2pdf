// SPDX-License-Identifier: MIT

// Package text contains conversions for the limited character set of PDF core fonts.
package text

import "strings"

// ToCP1252 replaces characters that PDF core fonts cannot write.
func ToCP1252(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '→':
			b.WriteString("->")
		case '←':
			b.WriteString("<-")
		case '↑':
			b.WriteByte('^')
		case '↓':
			b.WriteByte('v')
		case '≥':
			b.WriteString(">=")
		case '≤':
			b.WriteString("<=")
		case '≠':
			b.WriteString("!=")
		case '✓':
			b.WriteByte('v')
		case '✗':
			b.WriteByte('x')
		case '\u00a0':
			b.WriteByte(' ')
		default:
			if r >= 0x20 && r <= 0x7e || r >= 0xa0 && r <= 0xff {
				b.WriteByte(byte(r))
			} else if value, ok := inCP1252(r); ok {
				b.WriteByte(value)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

func inCP1252(r rune) (byte, bool) {
	switch r {
	case 0x20ac:
		return 0x80, true
	case 0x201a:
		return 0x82, true
	case 0x192:
		return 0x83, true
	case 0x201e:
		return 0x84, true
	case 0x2026:
		return 0x85, true
	case 0x2020:
		return 0x86, true
	case 0x2021:
		return 0x87, true
	case 0x2c6:
		return 0x88, true
	case 0x2030:
		return 0x89, true
	case 0x160:
		return 0x8a, true
	case 0x2039:
		return 0x8b, true
	case 0x152:
		return 0x8c, true
	case 0x17d:
		return 0x8e, true
	case 0x2018:
		return 0x91, true
	case 0x2019:
		return 0x92, true
	case 0x201c:
		return 0x93, true
	case 0x201d:
		return 0x94, true
	case 0x2022:
		return 0x95, true
	case 0x2013:
		return 0x96, true
	case 0x2014:
		return 0x97, true
	case 0x2dc:
		return 0x98, true
	case 0x2122:
		return 0x99, true
	case 0x161:
		return 0x9a, true
	case 0x203a:
		return 0x9b, true
	case 0x153:
		return 0x9c, true
	case 0x17e:
		return 0x9e, true
	case 0x178:
		return 0x9f, true
	}
	return 0, false
}
