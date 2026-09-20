# md2pdf

`md2pdf` converts Markdown to PDF. It has three modes of operation:

- One file to one PDF: `md2pdf testdata/sample.md`
- A directory merged into one PDF: `md2pdf testdata/pages`
- One PDF per file in a directory: `md2pdf --separate testdata/pages`

## Requirements

- Go 1.27 or later.
- [Mermaid CLI](https://github.com/mermaid-js/mermaid-cli) (`mmdc`) is optional and only needed to
  render Mermaid diagrams. Install it with npm:

  ```sh
  npm install -g @mermaid-js/mermaid-cli
  ```

  Without this renderer, Mermaid blocks appear as code blocks and nothing else unusual happens, so
  the tool stays usable without it.

## Building and installing

Build in the repository:

```sh
go build -o md2pdf ./cmd/md2pdf
```

Install to `$(go env GOPATH)/bin`:

```sh
go install github.com/gnutterts/md2pdf/cmd/md2pdf@latest
```

Run the tests:

```sh
go test ./...
```

The test suite does not require Mermaid CLI.

| Flag | Meaning |
|---|---|
| `-o <path>` | Output file, or the output directory with `--separate` |
| `--separate`, `-s` | Create one PDF for every Markdown file in a directory |
| `--mermaid <path>` | Path to the Mermaid renderer; `MD2PDF_MERMAID` can also set it, and `off` disables rendering |
| `--version` | Print the version |
| `-h`, `--help` | Print usage |

## Where the PDF goes

Without `-o`, output is placed next to the input:

| Input | Output |
|---|---|
| `md2pdf testdata/sample.md` | `testdata/sample.pdf` |
| `md2pdf testdata/pages` | `testdata/pages.pdf`, next to the directory |
| `md2pdf --separate testdata/pages` | one PDF next to every source file in `testdata/pages/` |

With `-o`, the path is a file name in the first two modes and a directory with `--separate`. An `-o`
that conflicts with this — an existing directory where a file name belongs, or an existing file where a
directory belongs — prints an error and exits with code 1.

For a directory, all `.md` files directly in that directory are included, not files in subdirectories,
in alphabetical file-name order. Names starting with `_` are skipped; a file with such a name that is
provided directly as an argument is still processed.

In merged mode, every source file starts on a new page. With `--separate`, an output directory that
does not yet exist is created.

## What goes into the PDF

Headings, paragraphs, bold, italic, inline code, links, nested and numbered lists, code blocks,
block quotes, thematic breaks, and tables. Tables have a bold header row, individually sized columns,
and a repeated header row when they span pages.

Text uses PDF base-14 fonts, which encode cp1252. Characters outside that encoding are transliterated
where possible — `→` becomes `->` — and otherwise replaced with a question mark.

Mermaid blocks are rendered with `mmdc` by default. Select another renderer with `--mermaid <path>`
or `MD2PDF_MERMAID`; `off` disables rendering. If the renderer is missing or fails, a warning is
written to stderr and the diagram remains visible as a code block.

## Known limitations

- Mermaid diagrams require a working external renderer.
- Inline formatting inside a table cell is flattened to plain text.
- Links are clickable but are not visibly distinguished from ordinary text.
- Code lines wider than the text column are truncated rather than wrapped.

## Dependency updates

[Renovate](https://docs.renovatebot.com) watches the Go modules and the GitHub Actions used here; its
configuration lives in [renovate.json](renovate.json). Updates wait seven days after release before a
pull request is opened, and Go modules are pinned to an exact version rather than a range.

## Licence

MIT; see [LICENSE](LICENSE).

`md2pdf` links against [goldmark](https://github.com/yuin/goldmark) and
[fpdf](https://github.com/go-pdf/fpdf), both under the MIT licence. Their notices are reproduced in
[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) and ship alongside the released binaries.
