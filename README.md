# md2pdf

`md2pdf` converts Markdown to PDF. It is a single binary: no browser, no LaTeX, and no runtime
dependencies.

- One file to one PDF: `md2pdf notes.md`
- A directory merged into one PDF: `md2pdf docs`
- One PDF per file in a directory: `md2pdf --separate docs`

## Installing

Download a binary for your platform from the [releases](https://github.com/gnutterts/md2pdf/releases),
or install with Go 1.27 or later:

```sh
go install github.com/gnutterts/md2pdf/cmd/md2pdf@latest
```

[Mermaid CLI](https://github.com/mermaid-js/mermaid-cli) (`mmdc`) is optional and only needed to render
Mermaid diagrams (`npm install -g @mermaid-js/mermaid-cli`). Without it, diagrams appear as code blocks.

## Documentation

- [Usage](docs/usage.md) — flags, where the PDF goes, how directories are read, Mermaid.
- [What goes into the PDF](docs/formatting.md) — supported Markdown, layout, and known limitations.
- [Development](docs/development.md) — building, testing, and dependency updates.
- [Changelog](CHANGELOG.md)

## Licence

MIT; see [LICENSE](LICENSE). `md2pdf` links against [goldmark](https://github.com/yuin/goldmark) and
[fpdf](https://github.com/go-pdf/fpdf), both under the MIT licence; their notices are in
[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) and ship alongside the released binaries.
