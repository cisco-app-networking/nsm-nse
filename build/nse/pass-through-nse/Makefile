NAME = pass-through-nse
CONTAINERS = pass-through-nse
PODS = pass-through-nse

VPP_AGENT_BASE_IMAGE = ligato/vpp-agent:v3.2.0

include $(TOP)/build/targets.mk

# If GOPATH is made up of several paths, use the first one for our targets in this Makefile
GO_TOP := $(shell echo ${GOPATH} | cut -d ':' -f1)
export GO_TOP
export OUT_DIR=$(GO_TOP)/out
BUILDTYPE_DIR:=debug
export PASS_THROUGH_OUT_LINUX:=$(OUT_DIR)/linux_amd64/$(BUILDTYPE_DIR)
# scratch dir for building isolated images
DOCKER_BUILD_TOP:=${PASS_THROUGH_OUT_LINUX}/docker_build

.PHONY: build-pass-through
build-pass-through:
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o ${GOPATH}/bin/linux_amd64/pass_through_nse ./cmd/pass-through-nse/...

.PHONY: docker-pass-through
docker-pass-through: build-pass-through
	mkdir -p ${DOCKER_BUILD_TOP}/pass-through/etc
	cp -p ./build/nse/pass-through-nse/Dockerfile.no_go_build_ucnf ${DOCKER_BUILD_TOP}/pass-through/
	cp -p ${GOPATH}/bin/linux_amd64/pass_through_nse ${DOCKER_BUILD_TOP}/pass-through/
	cp -pr ./build/nse/universal-cnf/vppagent/etc/* ${DOCKER_BUILD_TOP}/pass-through/etc/
	cd ${DOCKER_BUILD_TOP}/pass-through && docker build -t ${ORG}/pass-through-nse:${TAG} --build-arg VPP_AGENT=${VPP_AGENT_BASE_IMAGE} -f Dockerfile.no_go_build_ucnf .
