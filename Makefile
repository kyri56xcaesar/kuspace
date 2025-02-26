TARGET_API=cmd/userspace/
API_OUT=userspace
API_MAIN=main.go

TARGET_F_APP=cmd/frontendapp/
F_APP_OUT=frontendapp
F_APP_MAIN=main.go

TARGET_WS=cmd/frontendapp/ws/
WS_OUT=ws_server 
WS_MAIN=ws_main.go

TARGET_AUTH=cmd/minioth/
AUTH_OUT=minioth
AUTH_MAIN=main.go

TARGET_SHELL=cmd/shell/
SHELL_OUT=gshell
SHELL_MAIN=main.go

.PHONY: mod
mod:
	go mod tidy

.PHONY: all
all: build run

.PHONY: build
build:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN}
	go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}${F_APP_MAIN}
	go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}${AUTH_MAIN}
	go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}${WS_MAIN}

.PHONY: run
run: 
	./${TARGET_AUTH}${AUTH_OUT} &
	sleep 2
	./${TARGET_F_APP}${F_APP_OUT} & 
	sleep 2
	./${TARGET_API}${API_OUT} &
	sleep 2
	./${TARGET_WS}${WS_OUT} &

.PHONY: stop 
stop:
	kill $$(pgrep ${AUTH_OUT}) $$(pgrep ${WS_OUT}) $$(pgrep ${API_OUT}) $$(pgrep ${AUTH_OUT}) || true


.PHONY: userspace
userspace:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN} 
	./${TARGET_API}${API_OUT} 

.PHONY: front 
front:
	go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}${F_APP_MAIN}
	./${TARGET_F_APP}${F_APP_OUT} 

.PHONY: front-ws 
front-ws:
	go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}${WS_MAIN}
	./${TARGET_WS}${WS_OUT}

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
	rm -f ${TARGET_API}${API_OUT} ${TARGET_SHELL}${SHELL_OUT} ${TARGET_AUTH}${AUTH_OUT} ${TARGET_F_APP}${F_APP_OUT} ${TARGET_WS}${WS_OUT}
	
