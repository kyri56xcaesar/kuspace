TARGET_API=cmd/api/
API_OUT=api
API_MAIN=main.go

TARGET_AUTH=cmd/minioth/
AUTH_OUT=minioth
AUTH_MAIN=main.go

TARGET_SHELL=cmd/shell/
SHELL_OUT=gshell
SHELL_MAIN=main.go

.PHONY: mod
mod:
	go mod tidy


.PHONY: api
api:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN} 
	./${TARGET_API}${API_OUT}

.PHONY: minioth
minioth:
	go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}${AUTH_MAIN}
	./${TARGET_AUTH}${AUTH_OUT}

.PHONY: shell
shell:
	go build -o ${TARGET_SHELL}${SHELL_OUT} ${TARGET_SHELL}${SHELL_MAIN}
	./${TARGET_SHELL}${SHELL_OUT}

.PHONY: clean
clean:
	rm -f ${TARGET_API}${API_OUT} ${TARGET_SHELL}${SHELL_OUT}

