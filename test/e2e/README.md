# End-to-End Tests

This section contains information about end-to-end (e2e) tests for the VPP agent.

The e2e tests are written using Go. The e2e test suite can be executed for any of the supported VPP versions.

## Run E2E Test Suite

The easiest way to run the entire e2e test suite is to execute `test/e2e/run_e2e.sh` script.

### Run tests using the latest VPP release

```sh
# Run with latest VPP
./test/e2e/run_e2e.sh
```

### Run tests for a specific VPP version

Before running the tests, you must set the VPP_IMG variable to the desired image.

```sh
# Run with VPP 20.09
VPP_IMG=ligato/vpp-base:20.09 ./test/e2e/run_e2e.sh

# Run with VPP 20.01
VPP_IMG=ligato/vpp-base:20.01 ./test/e2e/run_e2e.sh
```

See https://github.com/ligato/vpp-base for all available versions.

## Customize E2E Test Run

This script supports the addition of extra arguments for the test run.

### Run specific tests cases

```sh
# Run test cases for IPSec
./tests/e2e/run_e2e.sh -test.run=Interface
```

### Run with any flags supported by `go test`

```sh
# Run tests in verbose mode
./tests/e2e/run_e2e.sh -test.v

# Run with CPU profiling
./tests/e2e/run_e2e.sh -test.cpuprofile "cpu.out"

# Run with memory profiling
./tests/e2e/run_e2e.sh -test.memprofile "mem.out" -test.memprofilerate=1
```
