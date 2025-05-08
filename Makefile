# ####
# A utility make file for compiling/running/generating_docs/cleaning this SOA 
# 
# runs on the system
# ####


# variables, only change these.
### services 
TARGET_API 		:= cmd/userspace/
API_OUT			:= userspace

TARGET_J_WS		:= cmd/userspace/jobs_feedback_ws/
J_WS_OUT		:= j_ws 

TARGET_F_APP	:= cmd/frontendapp/
F_APP_OUT		:= frontendapp

TARGET_WS		:= cmd/frontendapp/ws/
WS_OUT			:= ws_server 

TARGET_AUTH		:= cmd/minioth/
AUTH_OUT		:= minioth


# documantation related
# godoc
DOCS_DIR 				:= docs/

#Api docs (swagger)
API_DOCS_DIR :=api/
API_DOCS_USPACE_TARGET 		:= internal/userspace/api.go
API_DOCS_FRONTAPP_TARGET 	:= internal/frontendapp/api.go
API_DOCS_MINIOTH_TARGET 	:= pkg/minioth/minioth_server.go


.PHONY: api-docs
api-docs:
	swag init -g ${API_DOCS_USPACE_TARGET} --output ${API_DOCS_DIR}${API_OUT} --parseDependency --parseInternal
	swag init -g ${API_DOCS_FRONTAPP_TARGET} --output ${API_DOCS_DIR}${F_APP_OUT} --parseDependency --parseInternal
	swag init -g ${API_DOCS_MINIOTH_TARGET} --output ${API_DOCS_DIR}${AUTH_OUT} --parseDependency --parseInternal

# utility
.PHONY: clean
clean:
	rm -f ${TARGET_API}${API_OUT} ${TARGET_SHELL}${SHELL_OUT} ${TARGET_AUTH}${AUTH_OUT} ${TARGET_F_APP}${F_APP_OUT} ${TARGET_WS}${WS_OUT} ${TARGET_J_WS}${J_WS_OUT}
	
# perhaps useless
.PHONY: mod
mod:
	go mod tidy


.PHONY: all
all: build-all start-all

.PHONY: build-all
build-all:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}main.go
	go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}main.go
	go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}main.go
	go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}main.go
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}main.go


.PHONY: start-all
start-all: 
	./${TARGET_AUTH}${AUTH_OUT} &
	sleep 2
	./${TARGET_F_APP}${F_APP_OUT} & 
	sleep 2
	./${TARGET_API}${API_OUT} &
	sleep 2
	./${TARGET_WS}${WS_OUT} &
	sleep 2
	./${TARGET_J_WS}${J_WS_OUT} &


.PHONY: stop-all 
stop-all:
	kill $$(pgrep ${AUTH_OUT}) $$(pgrep ${WS_OUT}) $$(pgrep ${API_OUT}) $$(pgrep ${AUTH_OUT}) $$(pgrep ${J_WS_OUT}) $$(pgrep ${F_APP_OUT})|| true





# dirty each 
.PHONY: userspace
userspace:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}main.go
	./${TARGET_API}${API_OUT} 

.PHONY: j_ws
j_ws:
	go build -o ${TARGET_J_WS}${J_WS_OUT} ${TARGET_J_WS}main.go
	./${TARGET_J_WS}${J_WS_OUT}

.PHONY: front 
front:
	go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}main.go
	./${TARGET_F_APP}${F_APP_OUT} 

.PHONY: front-ws 
front-ws:
	go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}main.go
	./${TARGET_WS}${WS_OUT}

.PHONY: minioth
minioth:
	go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}main.go
	./${TARGET_AUTH}${AUTH_OUT} 

