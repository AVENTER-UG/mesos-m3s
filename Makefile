#Dockerfile vars

#vars
IMAGENAME=mesos-m3s
TAG=`git describe`
BUILDDATE=`date -u +%Y-%m-%dT%H:%M:%SZ`
IMAGEFULLNAME=avhost/${IMAGENAME}
BRANCH=`git symbolic-ref --short HEAD`
VERSION_URL=https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/${BRANCH}/.version.json

.PHONY: help build bootstrap all docs publish push version

help:
	    @echo "Makefile arguments:"
	    @echo ""
	    @echo "Makefile commands:"
			@echo "push"
	    @echo "build"
			@echo "build-bin"
	    @echo "all"
			@echo "docs"
			@echo "publish"
			@echo "version"
			@echo ${TAG}

.DEFAULT_GOAL := all

build:
	@echo ">>>> Build docker image"
	@docker buildx build --build-arg TAG=${TAG} --build-arg BUILDDATE=${BUILDDATE} --build-arg VERSION_URL=${VERSION_URL} -t ${IMAGEFULLNAME}:${BRANCH} .

build-bin:
	@echo ">>>> Build binary"
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.BuildVersion=${BUILDDATE} -X main.GitVersion=${TAG} -X main.VersionURL=${VERSION_URL} -extldflags \"-static\"" .

bootstrap:
	@echo ">>>> Build bootstrap"
	$(MAKE) -C $@

publish:
	@echo ">>>> Publish docker image"
	@docker buildx build --push --build-arg TAG=${TAG} --build-arg BUILDDATE=${BUILDDATE} --build-arg VERSION_URL=${VERSION_URL} -t ${IMAGEFULLNAME}:latest .

publish-tag:
	@echo ">>>> Publish docker image"
	@docker buildx build --push --build-arg TAG=${TAG} --build-arg BUILDDATE=${BUILDDATE} --build-arg VERSION_URL=${VERSION_URL} -t ${IMAGEFULLNAME}:${TAG} .

docs:
	@echo ">>>> Build docs"
	$(MAKE) -C $@

update-gomod:
	go get -u
	go mod tidy

seccheck:
	gosec --exclude G104 --exclude-dir ./vendor ./... 
	grype --add-cpes-if-none .

sboom:
	syft dir:. > sbom.txt
	syft dir:. -o json > sbom.json

go-fmt:
	@gofmt -w .
	@golangci-lint run --fix

version:
	@echo ">>>> Generate version file"
	@echo "{\"m3sVersion\": {	\"gitVersion\": \"${TAG}\",	\"buildDate\": \"${BUILDDATE}\"}}" > .version.json
	@cat .version.json
	@echo "Saved under .version.json"

check: go-fmt sboom seccheck
all: check version bootstrap build publish
