# Integration Tests

This section contains information about NSM-NSE integration tests for the VPP agent.

The integration tests are implemented as Go tests. They follow a pattern that is very similar to 
the [official e2e tests from Ligato vpp-agent](https://docs.ligato.io/en/latest/testing/end-to-end-tests) (`go.ligato.io/vpp-agent/v3/tests/e2e`).

## Running Tests

### Prerequisites

- [Docker 18.04+](https://docs.docker.com/engine/install)
- [Go 1.14+](https://golang.org/doc/install)

The easiest way to run the entire integration test suite is to execute `test/integration/run.sh` script.

Tests are executed using [gotestsum](https://github.com/gotestyourself/gotestsum).

### Run With Latest VPP-Agent

```sh
# Run using ligato/vpp-agent:latest image
./test/integration/run.sh
```

### Run With Specific VPP-Agent Version

Before running the tests, set the `VPP_AGENT` variable to the image you want to test against.

```sh
# Run with VPP 20.09
VPP_AGENT=ligato/vpp-agent:v3.2.0 ./test/integration/run.sh
```

### Run Specific Test Case

This script supports the addition of extra arguments for the test run.

```sh
# Run test cases containing AfPacket in name
./tests/integration/run.sh -test.run=AfPacket
```

### Run With Any Flags Supported By `go test`

```sh
# Run tests in verbose mode
./tests/integration/run.sh -test.v
```

## Writing Tests

When writing new test case, begin with calling `Setup()` in each test.

```go
func TestSomething(t *testing.T) {
    test := Setup(t)
    
    // testing..
}
```

The `Setup()` will take care of starting all the needed testing environment and return
_test context_ which provides methods to manage resources in the test or assert conditions.

```go
func TestSomething(t *testing.T) {
    // Start VPP-Agent container
    agent := test.SetupVPPAgent("agent")
    
    // Start Microservice container
    ms := test.StartMicroservice("microservice")
    
    // testing..
}
```

Check out test case `TestInterfaceAfPacket` in the [`010_interface_test.go`](010_interface_test.go) for simple test example that uses one agent and one microservice. For more advanced example, see test case `TestInterfaceAfPacketVNF` in the [`010_interface_test.go`](010_interface_test.go) which runs multi-agent setup.

The Gomega test framework is also ready to use for each test case, but not mandatory to use. See [Gomega online documentation](https://onsi.github.io/gomega/#making-assertions) for more info.

## References

For more info checkout Ligato documentation: https://docs.ligato.io/en/latest/
