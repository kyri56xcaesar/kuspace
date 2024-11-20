main_package_path = ./cmd/

.PHONY: mod
mod:
	go mod tidy

.PHONY: build all
build all: api shell


.PHONY: api
api:
	go build -o cmd/auther/api cmd/auther/main.go 
	./cmd/auther/api

.PHONY: shell
shell:
	go build -o cmd/shell/gshell cmd/shell/main.go
	./cmd/shell/gshell

