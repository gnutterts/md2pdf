# What goes into the PDF

## Supported Markdown

- **Headings**, six levels (levels 4 to 6 are 11 point bold, bold italic and italic), and **paragraphs**, including hard line breaks.
- **Inline formatting**: bold, italic, inline code, strike-through, backslash escapes, and HTML entities
  such as `&copy;` and `&#8594;`.
- **Links** are blue, underlined, and clickable; bare URLs, `www.` addresses, and e-mail addresses become
  links too.
- **Lists**: bulleted, numbered, and nested. An item may hold several paragraphs, code blocks, tables, or
  quotes, which stay aligned with its text. Task items (`- [ ]`, `- [x]`) get a drawn check box.
- **Code blocks** in Courier. Lines wider than the page wrap instead of being cut off.
- **Block quotes**, nested to any depth, with a grey bar per level; they may contain lists and code.
- **Tables** with a bold header row, individually sized columns, a header row that repeats on every
  page, and formatting and links inside cells.
- **Thematic breaks**.
- **Page numbers** (`n / N`, centred in grey below the text); turn them off with `--no-page-numbers`.
- **Table of contents** with `--toc`; see [Usage](usage.md#table-of-contents).
- **Bookmarks**: every heading becomes an entry in the outline of the PDF viewer.
- **Images** in PNG, JPEG or GIF, from `![alt](path)` or an HTML `<img src>`, with the path relative to the
  Markdown file. An image is drawn at its natural size (96 pixels to the inch) and never wider than the text.
  An image that is missing or in another format shows its alt text in italics, with a warning.
- **Mermaid diagrams**, rendered as images; see [Usage](usage.md#mermaid).
- **Front matter** in YAML (`---`), TOML (`+++`), or a Pandoc title block (`%`) is left out of the PDF; its `title` and `author` become the document title and author.
- **Raw HTML** contributes its text: `<br>` breaks the line, and the text of an HTML block — including
  `<img>` alt text and table cells — is kept. Comments, scripts, and styles are left out.

## Characters

Text uses the PDF base-14 fonts, which encode cp1252. Characters outside that encoding are
transliterated where possible — `→` becomes `->` — and otherwise replaced with a question mark.

## Known limitations

- Remote images (`https://…`), SVG and WebP show their alt text: md2pdf reads no network and fpdf cannot draw
  those formats. An image inside a heading, a table cell or a link also shows its alt text.
- Footnotes, definition lists, math, and emoji shortcodes appear as plain text.
- Formatting from HTML (bold, alignment, colour) is not applied; only its text is kept.
- Characters outside cp1252, such as Cyrillic, Greek, or CJK, become question marks.
- Mermaid diagrams require a working external renderer.
