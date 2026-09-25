// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"strings"
)

// Metadata holds the simple top-level fields of a front matter block.
type Metadata map[string]string

// Get returns the value of key, ignoring case; it is empty when the key is absent.
func (m Metadata) Get(key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// sourceLine is one line of source without its line terminator; end is the
// offset just past its terminator.
type sourceLine struct {
	end  int
	text []byte
}

// SplitFrontMatter removes a byte order mark and a front matter block from
// the start of source and returns its simple fields and the remaining Markdown.
func SplitFrontMatter(source []byte) (Metadata, []byte) {
	source = bytes.TrimPrefix(source, utf8BOM)
	lines := scanLines(source)
	if fields, consumed, ok := splitYAML(lines); ok {
		return fields, source[consumed:]
	}
	if fields, consumed, ok := splitTOML(lines); ok {
		return fields, source[consumed:]
	}
	if fields, consumed, ok := splitPandoc(lines); ok {
		return fields, source[consumed:]
	}
	return Metadata{}, source
}

func scanLines(source []byte) []sourceLine {
	var lines []sourceLine
	start := 0
	for {
		end := start
		for end < len(source) && source[end] != '\n' {
			end++
		}
		text := source[start:end]
		if len(text) > 0 && text[len(text)-1] == '\r' {
			text = text[:len(text)-1]
		}
		next := end
		if end < len(source) {
			next++
		}
		lines = append(lines, sourceLine{end: next, text: text})
		if end >= len(source) {
			return lines
		}
		start = next
	}
}

func splitYAML(lines []sourceLine) (Metadata, int, bool) {
	if len(lines) == 0 || !yamlOpen(lines[0].text) {
		return nil, 0, false
	}
	for i := 1; i < len(lines); i++ {
		if yamlClose(lines[i].text) {
			if !looksLikeYAML(lines[1:i]) {
				return nil, 0, false
			}
			return yamlFields(lines[1:i]), lines[i].end, true
		}
	}
	return nil, 0, false
}

// looksLikeYAML reports whether every top-level line is a key, so that a
// document opening with a thematic break does not lose the text up to the
// next one.
func looksLikeYAML(lines []sourceLine) bool {
	for _, line := range lines {
		text := line.text
		if len(bytes.TrimSpace(text)) == 0 || text[0] == ' ' || text[0] == '\t' || text[0] == '-' || text[0] == '#' {
			continue
		}
		index := bytes.IndexByte(text, ':')
		if index <= 0 || !isKey(bytes.TrimRight(text[:index], " \t")) {
			return false
		}
	}
	return true
}

func isKey(key []byte) bool {
	if len(key) == 0 {
		return false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

func yamlOpen(text []byte) bool {
	return fence(text) == "---"
}

// fence is a delimiter line without trailing blanks.
func fence(text []byte) string {
	return string(bytes.TrimRight(text, " \t"))
}

func yamlClose(text []byte) bool {
	return fence(text) == "---" || fence(text) == "..."
}

func yamlFields(lines []sourceLine) Metadata {
	meta := Metadata{}
	for _, line := range lines {
		text := line.text
		if len(text) == 0 || text[0] == ' ' || text[0] == '\t' || text[0] == '-' || text[0] == '#' {
			continue
		}
		key, value, ok := keyValue(text, ':')
		if !ok {
			continue
		}
		meta[string(key)] = string(value)
	}
	return meta
}

func splitTOML(lines []sourceLine) (Metadata, int, bool) {
	if len(lines) == 0 || fence(lines[0].text) != "+++" {
		return nil, 0, false
	}
	for i := 1; i < len(lines); i++ {
		if fence(lines[i].text) == "+++" {
			if !looksLikeTOML(lines[1:i]) {
				return nil, 0, false
			}
			return tomlFields(lines[1:i]), lines[i].end, true
		}
	}
	return nil, 0, false
}

// looksLikeTOML reports whether every top-level line is a key, a table
// header or a comment.
func looksLikeTOML(lines []sourceLine) bool {
	for _, line := range lines {
		text := line.text
		if len(bytes.TrimSpace(text)) == 0 || text[0] == ' ' || text[0] == '\t' || text[0] == '#' || text[0] == '[' {
			continue
		}
		index := bytes.IndexByte(text, '=')
		if index <= 0 || !isKey(bytes.TrimSpace(text[:index])) {
			return false
		}
	}
	return true
}

func tomlFields(lines []sourceLine) Metadata {
	meta := Metadata{}
	for _, line := range lines {
		text := line.text
		if len(text) == 0 || text[0] == ' ' || text[0] == '\t' {
			continue
		}
		if text[0] == '[' {
			break
		}
		key, value, ok := keyValue(text, '=')
		if !ok {
			continue
		}
		meta[string(key)] = string(value)
	}
	return meta
}

func splitPandoc(lines []sourceLine) (Metadata, int, bool) {
	if len(lines) == 0 || len(lines[0].text) == 0 || lines[0].text[0] != '%' {
		return nil, 0, false
	}
	fields := [...]string{"title", "author", "date"}
	meta := Metadata{}
	consumed := 0
	for i := 0; i < len(lines) && i < len(fields); i++ {
		text := lines[i].text
		if len(text) == 0 || text[0] != '%' {
			break
		}
		value := bytes.TrimSpace(text[1:])
		if len(value) > 0 {
			meta[fields[i]] = string(value)
		}
		consumed = lines[i].end
	}
	return meta, consumed, true
}

func keyValue(text []byte, separator byte) ([]byte, []byte, bool) {
	index := bytes.IndexByte(text, separator)
	if index < 0 {
		return nil, nil, false
	}
	key := bytes.TrimSpace(text[:index])
	value := bytes.TrimSpace(text[index+1:])
	if len(key) == 0 || len(value) == 0 {
		return nil, nil, false
	}
	return key, unquote(value), true
}

func unquote(value []byte) []byte {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}
