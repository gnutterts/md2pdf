# Changelog

## 0.4.0 — 2026-09-25

This release makes `md2pdf` behave predictably on input and renderers that do not cooperate.

### Added

- `--scale` (1 to 4, default 2, also `MD2PDF_SCALE`) sets how sharply Mermaid diagrams are rendered. A sharper
  diagram keeps the printed size it had before.
- `--mermaid-timeout` (default 30 seconds, also `MD2PDF_MERMAID_TIMEOUT`) sets the time limit per diagram.
- A diagram whose image is larger than 64 MiB is refused like a failed diagram: a warning, and the diagram
  stays visible as a code block.

### Changed

- Diagrams are rendered at scale 2 instead of 1, so they are no longer grainy on paper.
- A diagram that appears more than once, in one file or across the files of a directory, is rendered once.
- A panic is reported as `error: internal error: <reason>` with exit code 1 instead of a stack trace and
  exit code 2; set `MD2PDF_DEBUG=1` to print the stack as well.

### Fixed

- Deeply nested lists and quotes no longer leave no room for text: at least 120 points of text width remain,
  and the quote bars stay left of the text.

## 0.3.0 — 2026-09-25

This release makes the output a proper document, and makes runs safer.

### Added

- The PDF records a title, an author and the creator. The title comes from `--title`, the front matter,
  the first level 1 heading, or the file name (the directory name when a directory is merged); the author from `--author` or the front matter.
- Every page is numbered (`n / N`, centred in grey); `--no-page-numbers` turns it off.
- `--paper` (`a4`, `a5`, `a3`, `letter`, `legal`) and `--margin` (`20mm`, `0.75in`, `56pt`).
- Every heading becomes a bookmark in the outline of the PDF viewer.
- `--strict` treats a warning, such as a failed Mermaid diagram, as an error: the run stops, the failing file gets no PDF and the
  exit code is 1.
- `--force` allows `-o` to overwrite an existing file that is not a PDF.

### Changed

- Wrapped lines follow the font size, so long headings no longer overlap. Headings of level 5 and 6 are
  bold italic and italic.
- The default margin is 64 points instead of 56, which gives about 80 characters per line.
- With `--separate`, a file that cannot be read or written is skipped with a warning and the other files
  are still made; the run ends with `error: N of M files failed`.
- `go.mod` lists goldmark and fpdf as direct dependencies, and CI checks that the module is tidy and runs
  the tests with the race detector.

### Fixed

- `-o` naming an input file, also through a link or another spelling of the same path, is refused instead
  of overwriting the source.

## 0.2.1 — 2026-09-22

### Fixed

- Two adjacent lists of a different kind (a bulleted list immediately followed by a numbered one,
  or the reverse, with no blank paragraph between them) were drawn with no space between them and
  read as a single list.

## 0.2.0 — 2026-09-21

This release stops `md2pdf` from silently dropping or garbling content.

### Fixed

- Bare URLs, `www.` addresses, `<https://…>` autolinks, and e-mail addresses disappeared entirely;
  they are now shown as links, also inside table cells.
- Backslash escapes (`\*`) and HTML entities (`&copy;`, `&#8594;`) were printed literally.
- A hard line break printed a `?` in the middle of the sentence.
- A byte order mark at the start of a file printed a `?`.
- Front matter (YAML, TOML, or a Pandoc title block) was drawn as text.
- A list item with several paragraphs got a bullet for every paragraph, and code blocks, tables, and
  quotes inside an item started at the left margin.
- A list inside a quote was joined into a single line, and quotes lost their paragraphs and code.
- Code lines wider than the page were cut off, and very long lines took minutes to draw.
- Formatting and links inside table cells were flattened to plain text.
- `<br>` joined the surrounding words, and HTML blocks — including their text, image alt text, and
  table cells — disappeared.

### Added

- Links are blue and underlined.
- Strike-through (`~~text~~`) is drawn struck through.
- Task list items get a drawn check box.
- Block quotes get a grey bar per quote level.
- A list is followed by the same space as a paragraph.

### Changed

- The text of a list item starts at a fixed column, with the marker in a gutter before it.
- The README is shorter; the documentation lives in [docs/](docs/).

## 0.1.0 — 2026-09-21

First release: one file, a merged directory, or one PDF per file; headings, paragraphs, emphasis,
inline code, links, nested lists, code blocks, block quotes, thematic breaks, tables with repeating
header rows, and Mermaid diagrams through an external renderer.
