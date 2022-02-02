#Dockerfile vars

.PHONY: help build all

TAG=`git describe`
BUILDDATE=`date -u +%Y-%m-%dT%H:%M:%SZ`

help:
	    @echo "Makefile arguments:"
	    @echo ""
	    @echo "Makefile commands:"
	    @echo "build"
	    @echo "all"

.DEFAULT_GOAL := all

build:
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.BuildVersion=${BUILDDATE} -X main.GitVersion=${TAG} -extldflags \"-static\"" -o server server.go

all: build
