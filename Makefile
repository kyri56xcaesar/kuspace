# ####
# A utility make file for compiling/running/generating_docs/cleaning this SOA 
# 
# runs on the system
# ####



### services 
TARGET_API 		:= cmd/userspace/
API_OUT				:= userspace

TARGET_J_WS		:= cmd/userspace/jobs_feedback_ws/
J_WS_OUT			:= j_ws 

TARGET_F_APP	:= cmd/frontendapp/
F_APP_OUT			:= frontendapp

TARGET_WS			:= cmd/frontendapp/ws/
WS_OUT				:= ws_server 

TARGET_AUTH		:= cmd/minioth/
AUTH_OUT			:= minioth

TARGET_SHELL	:= cmd/shell/
SHELL_OUT			:= gshell



# documantation related
DOCS_DIR 							:= docs/
DOCS_USPACE_TARGET 		:= internal/userspace/api.go
DOCS_FRONTAPP_TARGET 	:= internal/frontendapp/api.go


.PHONY: gen-docs
gen-docs:
	swag init -g ${DOCS_USPACE_TARGET} --output ${DOCS_DIR}${API_OUT} --parseDependency --parseInternal
	swag init -g ${DOCS_FRONTAPP_TARGET} --output ${DOCS_DIR}${F_APP_OUT} --parseDependency --parseInternal



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

.PHONY: shell
shell:
	go build -o ${TARGET_SHELL}${SHELL_OUT} ${TARGET_SHELL}main.go
	./${TARGET_SHELL}${SHELL_OUT}


