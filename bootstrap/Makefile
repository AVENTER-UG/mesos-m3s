#Dockerfile vars

.PHONY: help build all

TAG=`git describe`
BUILDDATE=`date -u +%Y-%m-%dT%H:%M:%SZ`
BRANCH=`git symbolic-ref --short HEAD`
VERSION_URL=https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/${BRANCH}/bootstrap/.version.json


help:
	    @echo "Makefile arguments:"
	    @echo ""
	    @echo "Makefile commands:"
	    @echo "build"
	    @echo "all"

.DEFAULT_GOAL := all

build:
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.BuildVersion=${BUILDDATE} -X main.GitVersion=${TAG} -X main.VersionURL=${VERSION_URL} -extldflags \"-static\"" -o server server.go

version:
	@echo ">>>> Generate version file"
	@echo "[{\"bootstrapVersion\": {	\"gitVersion\": \"${TAG}\",	\"buildDate\": \"${BUILDDATE}\"}}]" > .version.json
	@cat .version.json
	@echo "Saved under .version.json"

gen-files:
	@echo ">>>> Generate Bootstrap and Update Script"
	@BRANCH=`git symbolic-ref --short HEAD` envsubst < bootstrap.ini > bootstrap.sh

all: version gen-files build
