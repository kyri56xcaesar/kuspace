module kuspace-web

// The browser code and its dev tools. This go.mod makes web/ its own (empty)
// module, so `go ... ./...` never walks into web/node_modules (npm packages
// can ship Go code: flatted does).
