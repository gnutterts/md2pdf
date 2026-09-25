// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"strings"
)

// htmlText extracts visible text from a raw HTML block and returns it as
// paragraphs. Formatting is ignored; only text, line breaks and cell
// separators are kept.
func htmlText(html string) []string { return htmlTextWith(html, nil) }

// imageMark stands in the paragraph list for an image that became its own part.
const imageMark = "\x00image\x00"

// htmlPart is a paragraph of text or, when src is set, an image.
type htmlPart struct {
	text string
	src  string
	alt  string
}

// htmlParts is htmlText that keeps an <img> whose src keep accepts as a part of
// its own, in document order, instead of turning it into its alt text.
func htmlParts(html string, keep func(src string) bool) []htmlPart {
	var images []htmlPart
	take := func(raw string) bool {
		src, ok := htmlAttribute(raw, "src")
		if !ok || !keep(src) {
			return false
		}
		alt, _ := htmlAttribute(raw, "alt")
		images = append(images, htmlPart{src: src, alt: alt})
		return true
	}
	var parts []htmlPart
	for _, text := range htmlTextWith(html, take) {
		if text == imageMark && len(images) > 0 {
			parts = append(parts, images[0])
			images = images[1:]
			continue
		}
		parts = append(parts, htmlPart{text: text})
	}
	return parts
}

func htmlTextWith(html string, image func(raw string) bool) []string {
	var paragraphs []string
	// inside counts the open tags in which an image stays text: a link, a table
	// cell or a heading, as it does in Markdown.
	inside := 0
	var current []byte
	for i := 0; i < len(html); {
		if html[i] != '<' {
			next := strings.IndexByte(html[i:], '<')
			if next < 0 {
				next = len(html) - i
			}
			appendHTMLText(&current, html[i:i+next])
			i += next
			continue
		}
		if strings.HasPrefix(html[i:], "<!--") {
			if end := strings.Index(html[i+4:], "-->"); end >= 0 {
				i += 4 + end + 3
			} else {
				i = len(html)
			}
			continue
		}
		end := htmlTagEnd(html, i)
		if end < 0 || !startsTag(html[i+1:]) {
			// A '<' that does not open a tag is text, like "5 < 6".
			appendHTMLText(&current, "<")
			i++
			continue
		}
		name, closing := htmlTagName(html[i+1 : end])
		if name == "script" || name == "style" {
			if !closing {
				if closeTag := strings.Index(strings.ToLower(html[end+1:]), "</"+name); closeTag >= 0 {
					i = end + 1 + closeTag
					continue
				}
				i = len(html)
				continue
			}
		} else {
			if keepsImageAsText(name) {
				if closing {
					inside = max(inside-1, 0)
				} else if !strings.HasSuffix(strings.TrimSpace(html[i+1:end]), "/") {
					inside++
				}
			}
			take := image
			if inside > 0 {
				take = nil
			}
			processHTMLTag(name, closing, html[i+1:end], &current, &paragraphs, take)
		}
		i = end + 1
	}
	flushHTMLParagraph(&current, &paragraphs)
	return paragraphs
}

// htmlTagEnd returns the index of the '>' that closes the tag starting at
// start, respecting quotes around attribute values.
func startsTag(rest string) bool {
	if rest == "" {
		return false
	}
	c := rest[0]
	return c == '/' || c == '!' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func htmlTagEnd(html string, start int) int {
	var quote byte
	for i := start + 1; i < len(html); i++ {
		switch c := html[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i
		}
	}
	return -1
}

func htmlTagName(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	closing := strings.HasPrefix(raw, "/")
	if closing {
		raw = strings.TrimSpace(raw[1:])
	}
	i := 0
	for i < len(raw) && isHTMLNameByte(raw[i]) {
		i++
	}
	return strings.ToLower(raw[:i]), closing
}

func isHTMLNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func processHTMLTag(name string, closing bool, raw string, current *[]byte, paragraphs *[]string, image func(raw string) bool) {
	switch name {
	case "br":
		appendHTMLBreak(current)
	case "img":
		if image != nil && image(raw) {
			flushHTMLParagraph(current, paragraphs)
			*paragraphs = append(*paragraphs, imageMark)
			return
		}
		if alt, ok := htmlAttribute(raw, "alt"); ok && alt != "" {
			appendHTMLText(current, alt)
		}
	case "td", "th":
		if !closing {
			appendHTMLCellSeparator(current)
		}
	default:
		if htmlParagraphTag(name) {
			flushHTMLParagraph(current, paragraphs)
		}
	}
}

func htmlParagraphTag(name string) bool {
	switch name {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol", "tr", "table",
		"details", "summary", "section", "article", "header", "footer", "blockquote", "pre", "hr":
		return true
	}
	return false
}

func appendHTMLText(current *[]byte, text string) {
	text = string(resolveReferences([]byte(text)))
	for i := 0; i < len(text); i++ {
		if isHTMLSpace(text[i]) {
			appendHTMLSpace(current)
		} else {
			*current = append(*current, text[i])
		}
	}
}

func isHTMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func appendHTMLSpace(current *[]byte) {
	if len(*current) == 0 {
		return
	}
	if last := (*current)[len(*current)-1]; last != ' ' && last != '\n' {
		*current = append(*current, ' ')
	}
}

func appendHTMLBreak(current *[]byte) {
	trimHTMLTrailingSpaces(current)
	if len(*current) > 0 {
		*current = append(*current, '\n')
	}
}

func appendHTMLCellSeparator(current *[]byte) {
	trimHTMLTrailingSpaces(current)
	if len(*current) > 0 {
		*current = append(*current, ' ', '|', ' ')
	}
}

func trimHTMLTrailingSpaces(current *[]byte) {
	for len(*current) > 0 && (*current)[len(*current)-1] == ' ' {
		*current = (*current)[:len(*current)-1]
	}
}

func flushHTMLParagraph(current *[]byte, paragraphs *[]string) {
	if text := strings.TrimSpace(string(*current)); text != "" {
		*paragraphs = append(*paragraphs, text)
	}
	*current = (*current)[:0]
}

// htmlAttribute returns the value of an HTML attribute from a tag's inner
// text. The value may use single or double quotes.
func htmlAttribute(raw, name string) (string, bool) {
	for i := 0; i < len(raw); {
		for i < len(raw) && isHTMLSpace(raw[i]) {
			i++
		}
		if i >= len(raw) {
			return "", false
		}
		start := i
		for i < len(raw) && !isHTMLSpace(raw[i]) && raw[i] != '=' {
			i++
		}
		attribute := raw[start:i]
		for i < len(raw) && isHTMLSpace(raw[i]) {
			i++
		}
		if i >= len(raw) || raw[i] != '=' {
			continue
		}
		i++
		for i < len(raw) && isHTMLSpace(raw[i]) {
			i++
		}
		if i < len(raw) && (raw[i] == '"' || raw[i] == '\'') {
			quote := raw[i]
			i++
			start = i
			for i < len(raw) && raw[i] != quote {
				i++
			}
			if i >= len(raw) {
				return "", false
			}
			if strings.EqualFold(attribute, name) {
				return raw[start:i], true
			}
			i++
			continue
		}
		start = i
		for i < len(raw) && !isHTMLSpace(raw[i]) {
			i++
		}
		if strings.EqualFold(attribute, name) {
			return raw[start:i], true
		}
	}
	return "", false
}

// isBreakTag reports whether a raw inline HTML tag is a line break.
func isBreakTag(tag []byte) bool {
	tag = bytes.TrimSpace(tag)
	if len(tag) < 3 || tag[0] != '<' || tag[len(tag)-1] != '>' {
		return false
	}
	inner := strings.TrimSpace(string(tag[1 : len(tag)-1]))
	i := 0
	for i < len(inner) && isHTMLNameByte(inner[i]) {
		i++
	}
	return i > 0 && strings.EqualFold(inner[:i], "br")
}

// keepsImageAsText reports whether an image inside this tag keeps its alt text.
func keepsImageAsText(name string) bool {
	switch name {
	case "a", "td", "th", "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}
