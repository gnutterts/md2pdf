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
| `--no-page-numbers` | Leave the page numbers out |
| `--version` | Print the version |
| `-h`, `--help` | Print usage |

Errors are written to stderr as `error: <reason>` and the exit code is 1.

The PDF also records `md2pdf <version>` as its creator. With `--separate`, `--title` and `--author` apply to
every file; without them each file gets its own title and author.

## Where the PDF goes

Without `-o`, output is placed next to the input:

| Input | Output |
|---|---|
| `md2pdf notes.md` | `notes.pdf` |
| `md2pdf docs` | `docs.pdf`, next to the directory |
| `md2pdf --separate docs` | one PDF next to every source file in `docs/` |

With `-o`, the path is a file name in the first two modes and a directory with `--separate`. An `-o`
that conflicts with this — an existing directory where a file name belongs, or an existing file where a
directory belongs — prints an error and exits with code 1.

## Directories

All `.md` files directly in the directory are included, not files in subdirectories, in alphabetical
file-name order. Names starting with `_` are skipped; a file with such a name that is provided directly
as an argument is still processed.

In merged mode, every source file starts on a new page. With `--separate`, an output directory that does
not yet exist is created.

## Mermaid

Mermaid blocks are rendered with `mmdc` by default. Select another renderer with `--mermaid <path>` or
`MD2PDF_MERMAID`; `off` disables rendering. If the renderer is missing or fails, a warning is written to
stderr, the diagram remains visible as a code block, and the exit code stays 0.
