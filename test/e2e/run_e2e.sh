#!/usr/bin/env bash
set -Eeuo pipefail

echo "Preparing vpp-agent end-to-end tests.."

args=($*)
VPP_IMG="${VPP_IMG:-ligato/vpp-base}"
testname="${NSMNSE_E2E_TEST_NAME:-vpp-agent-e2e-test}"
imgname="${NSMNSE_E2E_TEST_IMG:-vpp-agent-e2e-tests}"

# Compile vpp-agent for testing
go build -o ./test/e2e/vpp-agent.test \
    -tags 'osusergo netgo' \
    -ldflags '-w -s -extldflags "-static" -X go.ligato.io/vpp-agent/v3/pkg/version.app=vpp-agent.test-e2e' \
    go.ligato.io/vpp-agent/v3/cmd/vpp-agent

# Compile agentctl for testing
go build -o ./test/e2e/agentctl.test \
	  -tags 'osusergo netgo' \
    -ldflags '-w -s -extldflags "-static"' \
    go.ligato.io/vpp-agent/v3/cmd/agentctl

# Compile testing suite
go test -c -o ./test/e2e/e2e.test \
	  -tags 'osusergo netgo e2e' \
    -ldflags '-w -s -extldflags "-static"' \
    ./test/e2e

# Build testing image
docker build \
    -f ./test/e2e/Dockerfile.e2e \
    --build-arg VPP_IMG \
    --tag "${imgname}" \
    ./test/e2e

vppver=$(docker run --rm -i "$VPP_IMG" dpkg-query -f '${Version}' -W vpp)

cleanup() {
  echo "Removing the image.."
	set -x
	docker image rm "${imgname}" 2>/dev/null
}

trap 'cleanup' EXIT

echo "============================================================="
echo -e " E2E TEST - VPP \e[1;33m${vppver}\e[0m"
echo "============================================================="

# Run e2e tests
if docker run -i \
	--name "${testname}" \
	--rm \
	--pid=host \
	--privileged \
	--label io.ligato.vpp-agent.testsuite=e2e \
	--label io.ligato.vpp-agent.testname="${testname}" \
	--volume /var/run/docker.sock:/var/run/docker.sock \
	--env INITIAL_LOGLVL \
	${DOCKER_ARGS-} \
	"${imgname}" ${args[@]:-}
then
	echo >&2 "-------------------------------------------------------------"
	echo >&2 -e " \e[32mPASSED\e[0m (took: ${SECONDS}s)"
	echo >&2 "-------------------------------------------------------------"
	exit 0
else
	res=$?
	echo >&2 "-------------------------------------------------------------"
	echo >&2 -e " \e[31mFAILED!\e[0m (exit code: $res)"
	echo >&2 "-------------------------------------------------------------"
	exit $res
fi

