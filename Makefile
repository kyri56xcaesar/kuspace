TARGET_API=cmd/api/
API_OUT=api
API_MAIN=main.go
TARGET_SHELL=cmd/shell/
SHELL_OUT=gshell
SHELL_MAIN=main.go

.PHONY: mod
mod:
	go mod tidy

.PHONY: build all
build all: api shell


.PHONY: api
api:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN} 
	./${TARGET_API}${API_OUT}

.PHONY: shell
shell:
	go build -o ${TARGET_SHELL}${SHELL_OUT} ${TARGET_SHELL}${SHELL_MAIN}
	./${TARGET_SHELL}${SHELL_OUT}

