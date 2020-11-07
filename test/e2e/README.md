# End-to-End Tests

This section contains information about end-to-end (e2e) tests for the VPP agent.

The e2e tests are written as normal tests for Go. They follow same pattern as 
the [official end-to-end tests from Ligato vpp-agent](https://docs.ligato.io/en/latest/testing/end-to-end-tests) (`go.ligato.io/vpp-agent/v3/tests/e2e`). 

The e2e test suite can be executed for any of the supported VPP versions.

## Run E2E Test Suite

### Prerequisites

- [Docker 18.04+](https://docs.docker.com/engine/install)
- [Go 1.13+](https://golang.org/doc/install)

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

### Run specific tests cases

This script supports the addition of extra arguments for the test run.

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

## Writing new E2E test case

When writing new test case the minimal requirements are to call setup and teardown
for each test.

```go
func TestSomething(t *testing.T) {
	ctx := e2e.Setup(t)
	defer ctx.Teardown()

	// testing..
```

The `Setup()` will take care of starting all the needed infrastructure (fresh VPP, vpp-agent) and return
_test context_ which provides `Teardown()` method to clean up used resources.

The Gomega test framework is also ready to use for each test case, but not mandatory to use. See [Gomega online documentation](https://onsi.github.io/gomega/#making-assertions) for more info. 

Check out test case `TestInterfaceAfPacket` in the [`010_interface_test.go`]((010_interface_test.go) for sample code with separated commented sections.

## References

For more info checkout Ligato documentation: https://docs.ligato.io/en/latest/
