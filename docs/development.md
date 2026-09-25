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

## Releasing

1. In one pull request, raise `version` in `cmd/md2pdf/main.go` and add a `## <version> — <date>` section
   to `CHANGELOG.md`; merge it.
2. Check that the version is new: `https://sum.golang.org/lookup/github.com/gnutterts/md2pdf@v<version>` must
   answer 404. A published version number is never tagged again.
3. Tag the merge commit and push the tag: `git tag v<version> && git push origin v<version>`.

The release workflow then checks that the tag matches the version in the code, runs the tests, builds the
six binaries (darwin, linux and windows, amd64 and arm64), packs them with the licence, changelog, README and
documentation, writes `SHA256SUMS.txt`, and publishes the release with the changelog section as its notes.
