#Dockerfile vars

#vars
IMAGENAME=mesos-m3s
REPO=${registry}
TAG=`git describe`
BRANCH=`git rev-parse --abbrev-ref HEAD`
BUILDDATE=`date -u +%Y-%m-%dT%H:%M:%SZ`
IMAGEFULLNAME=${REPO}/${IMAGENAME}
IMAGEFULLNAMEPUB=avhost/${IMAGENAME}

.PHONY: help build bootstrap all

help:
	    @echo "Makefile arguments:"
	    @echo ""
	    @echo "Makefile commands:"
	    @echo "build"
			@echo "build-bin"
	    @echo "all"
			@echo "publish"
			@echo ${TAG}

.DEFAULT_GOAL := all

build:
	@echo ">>>> Build docker image and publish it to private repo"
	@docker buildx build --build-arg TAG=${TAG} --build-arg BUILDDATE=${BUILDDATE} -t ${IMAGEFULLNAME}:${BRANCH} --push .

build-bin:
	@echo ">>>> Build binary"
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.BuildVersion=${BUILDDATE} -X main.GitVersion=${TAG} -extldflags \"-static\"" .

bootstrap:
	@echo ">>>> Build bootstrap"
	$(MAKE) -C $@

publish:
	@echo ">>>> Publish docker image"
	@docker tag ${IMAGEFULLNAME}:${BRANCH} ${IMAGEFULLNAMEPUB}:${BRANCH}
	@docker push ${IMAGEFULLNAMEPUB}:${BRANCH}

docs:
	@echo ">>>> Build docs"
	$(MAKE) -C $@

version:
	@echo ">>>> Generate version file"
	@echo "[{ \"bootstrap_build\":\"${TAG}\", \"m3s_build\":\"${BUILDDATE}\", \"m3s_version\":\"${TAG}\"}]" > .version.json
	@cat .version.json
	@echo "Saved under .version.json"

all: bootstrap build version
