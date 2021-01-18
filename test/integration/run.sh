#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

args=($*)

echo "Preparing nsm-nse integration tests.."

export VPP_AGENT="${VPP_AGENT:-ligato/vpp-agent:latest}"
export TESTDATA_DIR="$SCRIPT_DIR/resources"
export GOTESTSUM_FORMAT="${GOTESTSUM_FORMAT:-testname}"

testname=${NSMNSE_E2E_TEST_NAME:-"nsmnse-integration-test"}
imgname=${NSMNSE_E2E_TEST_IMG:-"nsmnse-integration-tests"}

# TODO: make this configurable when upstream adds support
sharevolumename="share-for-vpp-agent-e2e-tests"

# Compile testing suite
go test -c -o ./test/integration/integration.test \
	  -tags 'osusergo netgo e2e' \
    -ldflags '-w -s -extldflags "-static"' \
    -trimpath \
    ./test/integration

# Build testing image
docker build \
    -f ./test/integration/Dockerfile \
    --tag "${imgname}" \
    ./test/integration

cleanup() {
    set +e

	  echo "Cleaning up nsm-nse integration tests.."

    # TODO: remove only resources that were started by this test

	  docker stop -t 1 "${testname}" 2>/dev/null
	  docker rm -v "${testname}" 2>/dev/null

    agents=$(docker ps -q --filter label="e2e.test.vppagent")
    if [ -n "$agents" ]; then
	      docker rm -f -v $agents 2>/dev/null
	  fi

    if [ "$(docker volume ls | grep "${sharevolumename}")" ]; then
        docker volume rm -f "${sharevolumename}"
    fi
}

trap 'cleanup' EXIT

echo -n "Creating volume for sharing files between containers.. "
if ! docker volume create "${sharevolumename}"; then
	  res=$?
	  echo >&2 -e "\e[31mvolume creation failed!\e[0m (exit code: $res)"
	  exit $res
fi

vppver=$(docker run --rm -i "$VPP_AGENT" dpkg-query -f '${Version}' -W vpp)

echo "=========================================================================="
echo -e " NSM-NSE INTEGRATION TESTS - $(date) "
echo "=========================================================================="
echo "-    VPP_AGENT: $VPP_AGENT"
echo "-     image ID: $(docker inspect $VPP_AGENT -f '{{.Id}}')"
echo "-      created: $(docker inspect $VPP_AGENT -f '{{.Created}}')"
echo "-  VPP version: $vppver"
echo "--------------------------------------------------------------------------"

# Run e2e tests
if docker run -i \
	--name "${testname}" \
	--pid=host \
	--privileged \
	--volume "${TESTDATA_DIR}":/testdata:ro \
	--volume /var/run/docker.sock:/var/run/docker.sock \
	--volume "${sharevolumename}":/test-share \
	--env INITIAL_LOGLVL \
	--env GOTESTSUM_FORMAT \
	--env TESTDATA_DIR \
	--env VPP_AGENT \
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
