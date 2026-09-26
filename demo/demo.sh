#!/usr/bin/env bash
#
# ArchUnitGo — 10-minute demo driver.
#
# Runs the hexagonal demo end to end: the green suite, a zoom into three rules,
# the generated dependency diagram, and the "break it and watch it bite" moment.
#
#   ./demo.sh            # interactive, waits for Enter between beats
#   DEMO_QUICK=1 ./demo.sh   # skip the pauses
#
set -euo pipefail

cd "$(dirname "$0")"

BOLD=$'\033[1m'
CYAN=$'\033[36m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
RESET=$'\033[0m'

QUICK="${DEMO_QUICK:-}"

say() { printf "\n${CYAN}${BOLD}%s${RESET}\n" "$1"; }
note() { printf "${YELLOW}%s${RESET}\n" "$1"; }
pause() {
  [[ -n "$QUICK" ]] && return 0
  printf "${BOLD}%s${RESET}" "${1:-Press Enter to continue…}"
  read -r _ || true
  echo
}
cmd() { printf "\n${GREEN}${BOLD}$ %s${RESET}\n" "$*"; "$@"; }

say "ArchUnitGo — a hexagonal architecture, kept honest by tests"
note "  Architecture rules written as ordinary go test functions."
note "  The dependency rule of the hexagon: adapters -> application -> domain, never backwards."
pause

say "1. The project"
cmd find internal cmd -name '*.go' | sort
note "  Four rings of the hexagon: domain (the core), application (the use cases),"
note "  adapters/primary (driving side), adapters/secondary (driven side)."
pause

say "2. The whole suite, in one command"
cmd go test ./... -v
note "  Nine rules pass. Each test is one rule, named the way the rule reads."
pause

say "3. The headline rule — the whole hexagon as one sentence"
cmd awk '/^func TestTheLayersFlowInwards/,/^}/' architecture/architecture_test.go
note "  Read it aloud: 'project layers, layer domain defined by folder internal/domain/**,'"
note "  'where layer domain may only depend on no layers,' ... and so on."
pause

cmd go test ./architecture -run TestTheLayersFlowInwards -v
note "  A named-layer policy: dependencies inside a layer are always allowed,"
note "  a dependency pointing at undeclared code (cmd/) is ignored."
pause

say "4. The same boundary, said two other ways"
cmd awk '/^func TestTheDomainDependsOnNoOtherFile/,/^}/' architecture/architecture_test.go
pause
cmd awk '/^func TestTheDomainSliceDoesNotDependOnTheAdaptersSlice/,/^}/' architecture/architecture_test.go
note "  Same idea, different vocabulary: files vs slices. Pick the one that fits."
pause

say "5. A rule is a value, not an action"
note "  Building a chain reads nothing; only the terminal check touches the project."
note "  And a selector that matches nothing fails — a stale glob must not pass silently."
pause

say "6. The dependency graph, drawn for you"
cmd go run ./cmd/diagram
cmd cat docs/architecture.mmd
note "  Mermaid renders on GitHub; docs/architecture.html is self-contained."
pause

say "7. The demo moment — break it and watch it bite"
note "  I am about to add a forbidden import to the domain, run the tests, and"
note "  then put the file back exactly as it was."
pause

cp internal/domain/order.go internal/domain/order.go.demo-bak
trap 'cp internal/domain/order.go.demo-bak internal/domain/order.go 2>/dev/null || true; rm -f internal/domain/order.go.demo-bak' EXIT

perl -0pi -e 's{package domain}{package domain\n\nimport "github.com/LukasNiessen/ArchUnitGoDemo/internal/application"\n\n// TEMP: deliberate violation to demo ArchUnitGo catching it\nvar _ = application.PlaceOrder{}}' internal/domain/order.go

note "  (domain/order.go now imports the application layer — backwards)"
pause

if go test ./architecture -run TestTheLayersFlowInwards -v; then
  note "  Unexpected: the broken rule passed."
else
  note "  Three rules now fail, each naming the exact import that broke them."
fi
pause

cp internal/domain/order.go.demo-bak internal/domain/order.go
rm -f internal/domain/order.go.demo-bak
trap - EXIT

note "  (order.go restored)"
cmd go test ./... -v
note "  Green again. That is the whole demo."
pause

say "Done"
note "  Full walkthrough and copy-paste break-it instructions: demo/README.md"
