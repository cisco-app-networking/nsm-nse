package integration_test

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()

	if strings.ToLower(os.Getenv("TEST")) != "integration" {
		fmt.Fprintln(os.Stderr, "integration tests must be executed with env var TEST=integration")
		os.Exit(0)
	}

	r := m.Run()
	os.Exit(r)
}
