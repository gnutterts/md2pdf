# Development

Build in the repository:

```sh
go build -o md2pdf ./cmd/md2pdf
```

Run the tests:

```sh
go test ./...
```

The test suite does not require Mermaid CLI; it uses a fake renderer. CI also runs the tests with the race
detector and fails when `go.mod` or `go.sum` is not tidy (`go mod tidy -diff`).

## Dependency updates

[Renovate](https://docs.renovatebot.com) watches the Go modules and the GitHub Actions used here; its
configuration lives in [renovate.json](../renovate.json). Updates wait seven days after release before
a pull request is opened, and Go modules are pinned to an exact version rather than a range.
