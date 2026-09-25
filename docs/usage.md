# Usage

```sh
md2pdf [flags] <file.md | directory>
```

| Flag | Meaning |
|---|---|
| `-o <path>` | Output file, or the output directory with `--separate` |
| `--separate`, `-s` | Create one PDF for every Markdown file in a directory |
| `--mermaid <path>` | Path to the Mermaid renderer; `MD2PDF_MERMAID` can also set it, and `off` disables rendering |
| `--title <text>` | Document title; by default the `title` in the front matter, else the first level 1 heading, else the file name (the directory name when a directory is merged) |
| `--author <text>` | Document author; by default the `author` in the front matter |
| `--scale <n>` | Sharpness of Mermaid diagrams, from 1 to 4; default 2. `MD2PDF_SCALE` can also set it |
| `--mermaid-timeout <duration>` | Time limit per diagram, such as `45s` or `2m`; default `30s`. `MD2PDF_MERMAID_TIMEOUT` can also set it |
| `--font <dir>` | Use your own font for text instead of DejaVu Sans; see [Fonts](#fonts). `MD2PDF_FONT` can also set it |
| `--font-mono <dir>` | The same for code, instead of DejaVu Sans Mono. `MD2PDF_FONT_MONO` can also set it |
| `--toc` | Start the PDF with a clickable table of contents |
| `--toc-depth <n>` | Deepest heading level in the table of contents, 1 to 6; default 3. Implies `--toc` |
| `--strict` | Treat every warning as an error: stop at the first one, write no PDF, and exit with code 1 |
| `--force` | Allow `-o` to overwrite an existing file that is not a PDF |
| `--no-page-numbers` | Leave the page numbers out |
| `--paper <size>` | Paper size: `a4` (default), `a5`, `a3`, `letter` or `legal`, portrait |
| `--margin <length>` | Margin on every side, such as `20mm`, `0.75in` or `56pt`; a bare number is in points. Default `64pt`; at most a quarter of the shortest side of the paper |
| `--version` | Print the version |
| `-h`, `--help` | Print usage |

Errors are written to stderr as `error: <reason>` and the exit code is 1. An unexpected failure inside md2pdf
is reported as `error: internal error: <reason>`, also with exit code 1; set `MD2PDF_DEBUG=1` to print the
stack as well.

The PDF also records `md2pdf <version>` as its creator. With `--separate`, `--title` and `--author` apply to
every file; without them each file gets its own title and author.

## Where the PDF goes

Without `-o`, output is placed next to the input:

| Input | Output |
|---|---|
| `md2pdf notes.md` | `notes.pdf` |
| `md2pdf docs` | `docs.pdf`, next to the directory |
| `md2pdf --separate docs` | one PDF next to every source file in `docs/` |

An existing PDF is overwritten. An `-o` that names one of the input files is always refused, and one that
names an existing file that does not end in `.pdf` is refused unless you add `--force`.

With `-o`, the path is a file name in the first two modes and a directory with `--separate`. An `-o`
that conflicts with this — an existing directory where a file name belongs, or an existing file where a
directory belongs — prints an error and exits with code 1.

## Directories

All `.md` files directly in the directory are included, not files in subdirectories, in alphabetical
file-name order. Names starting with `_` are skipped; a file with such a name that is provided directly
as an argument is still processed.

In merged mode, every source file starts on a new page. With `--separate`, an output directory that does
not yet exist is created. A file that cannot be read or written is skipped with a warning and the other
files are still made; the run then ends with `error: 1 of 13 files failed` and exit code 1. With `--strict`
the batch stops at the first failure.

## Fonts

Text is set in DejaVu Sans and code in DejaVu Sans Mono, both built into `md2pdf`. For characters those fonts
lack, such as Chinese, Japanese or Korean, or for a house style, point `--font` (text) or `--font-mono` (code)
at a directory with TrueType files named:

| File | Used for |
|---|---|
| `Regular.ttf` | Required: ordinary text |
| `Bold.ttf` | Bold text and headings |
| `Italic.ttf` | Italic text and quotes |
| `BoldItalic.ttf` | Bold italic text |

A missing style falls back to `Regular.ttf`, so without `Bold.ttf` headings are not bold. Only TrueType
outlines can be embedded: most `.otf` files use PostScript outlines and are refused with a message. Only the
characters a document uses end up in the PDF, so even a large font adds little to it. For example, Noto Sans
CJK, converted to TrueType, covers Chinese, Japanese and Korean.

## Table of contents

With `--toc`, the PDF starts with a page titled *Contents* that lists the headings with their page numbers;
every entry links to its heading. In a merged directory there is one table for the whole document; with
`--separate`, every PDF gets its own. Page numbers are only known once the document is laid out, so the
document is drawn twice, which roughly doubles the time for large documents (diagrams are not rendered
twice). A document without headings up to the chosen level gets no table and a warning.

## Mermaid

A diagram that appears more than once, in one file or across the files of a directory, is rendered once.
Up to four diagrams are rendered at the same time (fewer on a machine with fewer processors), before the PDF
is drawn.

Mermaid blocks are rendered with `mmdc` by default. Select another renderer with `--mermaid <path>` or
`MD2PDF_MERMAID`; `off` disables rendering. A diagram whose image is larger than 64 MiB is refused like a failed one. If the renderer is missing or fails, a warning is written to
stderr, the diagram remains visible as a code block, and the exit code stays 0. With `--strict` the warning
is an error instead, and no PDF is written.
