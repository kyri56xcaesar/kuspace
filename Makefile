main_package_path = ./cmd/

.PHONY: build all
build all:


.PHONY: api
api:
	go build -o cmd/auther/api cmd/auther/main.go 
	./cmd/auther/api

