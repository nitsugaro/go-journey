package main

import (
	"os/exec"
	"testing"
)

// TestJourneyConfigurations executes the same configuration-driven scenario
// runner used by `go run .`. Keeping the scenarios in JSON means this test only
// acts as the bridge that makes `go test ./...` prove those journeys work.
func TestJourneyConfigurations(t *testing.T) {
	command := exec.Command("go", "run", ".")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("journey scenarios failed:\n%s\n%v", output, err)
	}
	t.Logf("journey scenarios:\n%s", output)
}
