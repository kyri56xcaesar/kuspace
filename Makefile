# ##### #
# [DEV] #
# ##### #
#
# Going to migrate to a Go tool for this, but for now, this is a make file to help
#
# A utility make file for compiling/running/generating_docs/cleaning this SOA 
# locally.
# 
# runs on this system
# ####


# variables, only change these.
### services 
TARGET_API 		:= cmd/uspace/
API_OUT			:= uspace

TARGET_WSS		:= cmd/wss/
WSS_OUT			:= wss 

TARGET_F_APP	:= cmd/frontapp/
F_APP_OUT		:= frontapp

TARGET_AUTH		:= cmd/minioth/
AUTH_OUT		:= minioth


# documantation related
# golds
# swag
.PHONY: api-docs code-docs clean all build-all start-all stop-all uspace front wss minioth lint

api-docs:
	swag init -g internal/uspace/api.go -o api/uspace --instanceName uspacedocs --exclude pkg/fslite,pkg/minioth --parseDependency --parseInternal
	swag init -g pkg/minioth/minioth_server.go -o api/minioth --instanceName miniothdocs --exclude pkg/fslite,internal/uspace --parseDependency --parseInternal
	swag init -g pkg/fslite/fslite_server.go -o api/fslite --instanceName fslitedocs --exclude pkg/minioth,internal/uspace --parseDependency --parseInternal

code-docs:
	golds -gen -dir docs/minioth -compact -wdpkgs-listing solo .\pkg\minioth\
	golds -gen -dir docs/fslite -compact -wdpkgs-listing solo .\pkg\fslite\
	golds -gen -dir docs/uspace -compact -wdpkgs-listing solo .\internal\uspace\

# utility
clean:
	rm -f ${TARGET_API}${API_OUT} ${TARGET_SHELL}${SHELL_OUT} ${TARGET_AUTH}${AUTH_OUT} ${TARGET_F_APP}${F_APP_OUT}  ${TARGET_WSS}${WSS_OUT}
	rm api/uspace/* api/minioth/* api/fslite/*
	rm -rf docs/minioth/*
	rm -rf docs/fslite/*
	rm -rf docs/uspace/*
	
all: build-all start-all

build-all:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}main.go
	go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}main.go
	go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}main.go
	go build -o ${TARGET_WSS}${WSS_OUT} ${TARGET_WSS}main.go


start-all: 
	./${TARGET_AUTH}${AUTH_OUT} &
	sleep 2
	./${TARGET_F_APP}${F_APP_OUT} & 
	sleep 2
	./${TARGET_API}${API_OUT} &
	sleep 2
	./${TARGET_WSS}${WSS_OUT} &


stop-all:
	kill $$(pgrep ${AUTH_OUT}) $$(pgrep ${API_OUT}) $$(pgrep ${WSS_OUT}) $$(pgrep ${F_APP_OUT})|| true



# dirty each 
uspace:
	go build -o ${TARGET_API}${API_OUT} ${TARGET_API}main.go
	./${TARGET_API}${API_OUT} 

wss:
	go build -o ${TARGET_WSS}${WSS_OUT} ${TARGET_WSS}main.go
	./${TARGET_WSS}${WSS_OUT}

front:
	go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}main.go
	./${TARGET_F_APP}${F_APP_OUT} 

minioth:
	go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}main.go
	./${TARGET_AUTH}${AUTH_OUT} 


# must install lint 
lint:
	golangci-lint run ./... -c ./.golangci-lint.yaml