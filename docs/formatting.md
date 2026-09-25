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

Text is set in DejaVu Sans and code in DejaVu Sans Mono. Both are embedded in `md2pdf` and in every PDF,
where only the characters that are used are included, so a PDF grows by some tens of kilobytes. They cover
Latin, Greek and Cyrillic scripts, arrows, and many mathematical and other symbols.

## Known limitations

- Remote images (`https://…`), SVG and WebP show their alt text: md2pdf reads no network and fpdf cannot draw
  those formats. An image inside a heading, a table cell or a link also shows its alt text.
- Footnotes, definition lists, math, and emoji shortcodes appear as plain text.
- Formatting from HTML (bold, alignment, colour) is not applied; only its text is kept.
- Characters the fonts do not have, such as Chinese, Japanese, Korean or emoji, show as an empty box.
  Arabic and Hebrew are not shaped or set right to left.
- Mermaid diagrams require a working external renderer.
