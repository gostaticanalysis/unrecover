# unrecover

[![pkg.go.dev][gopkg-badge]][gopkg]

`unrecover` finds a calling function in other goroutine which does not recover any panic.

```go
func f() {
	go func() { // want "this goroutine does not recover a panic"
	}()

	go func() { // OK
		defer func() { recover() }()
	}()

	go func() { // want "this goroutine does not recover a panic"
		recover()
	}()
}
```

## Install

You can get `unrecover` by `go install` command (Go 1.16 and higher).

```bash
$ go install github.com/gostaticanalysis/unrecover/cmd/unrecover@latest
```

## How to use

`unrecover` run with `go vet` as below when Go is 1.12 and higher.

```bash
$ go vet -vettool=$(which unrecover) ./...
```

## Analyze with golang.org/x/tools/go/analysis

You can use [unrecover.Analyzer](https://pkg.go.dev/github.com/gostaticanalysis/unrecover/#Analyzer) with [unitchecker](https://golang.org/x/tools/go/analysis/unitchecker).

<!-- links -->
[gopkg]: https://pkg.go.dev/github.com/gostaticanalysis/unrecover
[gopkg-badge]: https://pkg.go.dev/badge/github.com/gostaticanalysis/unrecover?status.svg
