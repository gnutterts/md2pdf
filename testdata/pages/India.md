# Page layout

This page is test material: it exists so the renderer has something with margins, line height and where a page break lands to draw.

## What this page covers

Page layout is one of eleven pages in this folder. Together they are merged into a single document by
one of the output modes, which is how page breaks between files are checked.

- A list item about margins, line height and where a page break lands
- A second item, written long enough that it wraps onto the following line and shows whether the
  continuation stays aligned under the text rather than under the bullet
- A third item

## An example

```text
md2pdf testdata/pages --separate
```

| Aspect | Value |
|---|---|
| Page | Page layout |
| Kind | test material |

> Every page ends with a quote so the merged document contains several of them.
