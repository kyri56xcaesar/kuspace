module kuspace-runtime-data

// Runtime data written by the services and containers. This go.mod makes
// data/ its own (empty) module, so `go build ./...` never walks into it -
// containers leave root-owned directories here that it cannot read.
