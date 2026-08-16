package archunit_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// This file is the workflow in .github/workflows/ci.yml held to being the gate, by the same argument
// architecture_test.go makes about the architecture: a check somebody has to remember to keep is a
// check that leaves the day somebody is in a hurry. Dropping `-race`, or `-shuffle=on`, or the
// `config verify` that would notice .golangci.yml had stopped parsing, makes CI faster and greener and
// nothing fails — which is the failure mode the whole library exists to complain about.
//
// The workflow is read as text and not parsed, because parsing YAML needs a dependency this module
// does not have and will not take for a test. It is either collapsed to one line or read line by line
// first, so that nothing below says anything about how the file is indented or where it wraps; what
// they pin is that each command of the gate is in there, spelled the way a maintainer types it.

// ciWorkflow is the one workflow of this repository. A second one would be a check somewhere the tests
// below say nothing about, which is why the path is spelled here once.
const ciWorkflow = ".github/workflows/ci.yml"

// gateOfThisRepository is every check a change has to pass, as the command that runs it. It is the list
// in the contributor's terminal and in the CI file at once — that is the point of it.
//
// The two `go test` lines are one command with two scopes, and both are here: `./...` is the suite, and
// `.` on its own is the root package, where the dogfooding architecture suite lives and where the CI
// file gives it a job of its own.
func gateOfThisRepository() []string {
	return []string{
		"go build ./...",
		"go vet ./...",
		"go mod tidy -diff",
		"go test -race -shuffle=on -count=1 ./...",
		"go test -race -shuffle=on -count=1 .",
		"golangci-lint config verify",
		"golangci-lint fmt --diff",
		"golangci-lint run ./...",
	}
}

// TestTheCiWorkflowRunsEveryCheckOfTheGate is the guard on the list above: every command of the gate is
// a step of the workflow, run as itself rather than folded into an action's arguments where a reader
// cannot see it and this test cannot find it.
func TestTheCiWorkflowRunsEveryCheckOfTheGate(t *testing.T) {
	commands := commandsOfTheCiWorkflow(t)

	for _, command := range gateOfThisRepository() {
		if !slices.Contains(commands, command) {
			t.Errorf("%s runs %v, and none of them is %q: every check of the gate is a step of the workflow",
				ciWorkflow, commands, command)
		}
	}
}

// TestTheCiWorkflowBuildsForTheOtherPlatformsToo is the half of the gate that is not a command: the two
// platforms this library is compiled for and cannot be tested on. Windows is where a path separator
// that escaped normalisation lands, since every rule matches its patterns against identifiers built
// from paths; 386 is where a calculation that assumed a 64-bit int stops compiling.
func TestTheCiWorkflowBuildsForTheOtherPlatformsToo(t *testing.T) {
	workflow := collapsed(readCiWorkflow(t))

	for _, target := range []string{"goos: windows goarch: amd64", "goos: linux goarch: '386'"} {
		if !strings.Contains(workflow, target) {
			t.Errorf("%s does not build for %q: the matrix is what says this library is not linux/amd64 only",
				ciWorkflow, target)
		}
	}
	// The matrix only means anything if the build reads it, and a job that named a platform without
	// exporting it would build the runner's own twice.
	for _, exported := range []string{"GOOS: ${{ matrix.goos }}", "GOARCH: ${{ matrix.goarch }}"} {
		if !strings.Contains(workflow, exported) {
			t.Errorf("%s names the platforms of its matrix but does not set %s from it", ciWorkflow, exported)
		}
	}
}

// TestTheCiWorkflowRunsOnEveryPush pins the trigger. A workflow narrowed to `main`, or to pull requests,
// is a gate this repository would walk straight past: AGENTS.md has the maintainer committing straight
// to main with no pull request, so a branch nobody proposed is still the code.
func TestTheCiWorkflowRunsOnEveryPush(t *testing.T) {
	workflow := collapsed(readCiWorkflow(t))

	if !strings.Contains(workflow, "push: branches: ['**']") {
		t.Errorf("%s is not triggered by a push to every branch", ciWorkflow)
	}
	for _, event := range []string{"pull_request:", "workflow_dispatch:"} {
		if !strings.Contains(workflow, event) {
			t.Errorf("%s is not triggered by %s", ciWorkflow, event)
		}
	}
}

// TestTheCiWorkflowCannotGoGreenWithACheckSwitchedOff is the reason this file exists rather than a line
// of prose in a README. A red check is a change nobody has finished; the ways to make it green without
// finishing are all one keyword, and each of them leaves the file looking exactly as it does now.
func TestTheCiWorkflowCannotGoGreenWithACheckSwitchedOff(t *testing.T) {
	workflow := collapsed(readCiWorkflow(t))

	// continue-on-error reports the failure and passes the job anyway. `if: false` is a step that never
	// runs at all. Both are a deleted check that reads as a configured one.
	for _, switchedOff := range []string{"continue-on-error", "if: false"} {
		if strings.Contains(workflow, switchedOff) {
			t.Errorf("%s uses %q: a check that cannot fail the build is not a check", ciWorkflow, switchedOff)
		}
	}
	// A floating linter turns somebody else's release into this repository's red build, and — worse — a
	// rule that was dropped upstream into a check that silently stopped running here.
	if !strings.Contains(workflow, "version: v2.") || strings.Contains(workflow, "version: latest") {
		t.Errorf("%s does not pin an exact golangci-lint v2 version", ciWorkflow)
	}
	// The linter's own config is the deterministic half of code review, so the file failing to parse has
	// to fail the build rather than turn the review off. `golangci-lint config verify` is that read, and
	// it is in the gate above; here it is pinned as the first of the three lint commands, which is the
	// order in which it reports the problem itself instead of letting the run report it obliquely.
	commands := commandsOfTheCiWorkflow(t)
	verify := slices.Index(commands, "golangci-lint config verify")
	run := slices.Index(commands, "golangci-lint run ./...")
	if verify < 0 || run < 0 || verify > run {
		t.Errorf("%s runs %v: .golangci.yml is verified before it is linted with", ciWorkflow, commands)
	}
}

// readCiWorkflow returns the workflow file's contents, failing the test if the one file every check of
// this repository lives in is not there.
func readCiWorkflow(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(strings.Split(ciWorkflow, "/")...))
	if err != nil {
		t.Fatalf("reading %s failed: %v", ciWorkflow, err)
	}
	return string(content)
}

// commandsOfTheCiWorkflow returns every command the workflow runs, in the order they appear: the value
// of each `run:` key, whether the step was named or written as a bare `- run:`.
func commandsOfTheCiWorkflow(t *testing.T) []string {
	t.Helper()

	var commands []string
	for line := range strings.Lines(readCiWorkflow(t)) {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		if command, isRun := strings.CutPrefix(trimmed, "run:"); isRun {
			commands = append(commands, strings.TrimSpace(command))
		}
	}
	if len(commands) == 0 {
		t.Fatalf("%s runs no command at all", ciWorkflow)
	}
	return commands
}

// collapsed is the workflow with every run of whitespace turned into one space, so that a test asking
// what the file says is not also asserting how it is indented or where its lines were wrapped.
func collapsed(workflow string) string {
	return strings.Join(strings.Fields(workflow), " ")
}
