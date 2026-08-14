package fluentapi_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// TestEveryTerminalOfThisModuleGuardsAgainstAnEmptyTest is the guard held to the word `every`: not the four
// terminals a reader thought of, but every Checkable the grammar of this module can reach, found by walking
// the method sets rather than by listing them. A predicate that lands later is swept up by the same walk, so
// forgetting to wire the guard into it fails here instead of passing forever in a user's build.
//
// Each terminal is checked twice against a project whose files none of them selected: by default, where the
// stale pattern has to be reported as an empty test, and with AllowEmptyTests, where the user has said they
// meant it and the rule has to pass.
func TestEveryTerminalOfThisModuleGuardsAgainstAnEmptyTest(t *testing.T) {
	// A scope naming a folder the fixture does not have: the typo the guard exists for, and the one thing
	// every terminal below has in common.
	scope := fluentapi.ProjectFiles(fixtureLocator(t, writeFixtureProject(t))).InFolder("internal/apis/**")
	terminals := terminalsOf(t, scope.Should(), scope.ShouldNot())

	for _, name := range []string{
		"HaveNoCycles", "HaveName", "BeInFolder", "BeInPath", "DependOnFiles", "DependOnExternalModules", "AdhereTo",
	} {
		if !slices.ContainsFunc(terminals, func(found terminal) bool { return strings.Contains(found.sentence, name) }) {
			t.Fatalf("the walk found %d terminals and none of them is %s: it has gone blind, and the sweep below proves nothing", len(terminals), name)
		}
	}

	for _, found := range terminals {
		t.Run(found.sentence, func(t *testing.T) {
			violations, err := found.checkable.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", found.sentence, err)
			}
			if len(violations) == 0 {
				t.Fatalf("%s passes on a scope that selected no file, want the empty-test violation: the guard is not wired in", found.sentence)
			}
			for index, violation := range violations {
				empty, ok := violation.(assertion.EmptyTestViolation)
				if !ok {
					t.Fatalf("%s reports a %T, want EmptyTestViolations alone: the rule was judged instead of guarded", found.sentence, violation)
				}
				if kind := empty.Kind(); kind != assertion.KindEmptyTest {
					t.Errorf("violation %d is of kind %q, want %q", index, kind, assertion.KindEmptyTest)
				}
				if empty.Subject == "" {
					t.Errorf("violation %d names no vocabulary, so a report cannot say what the rule selected nothing of", index)
				}
				if len(empty.Selectors) == 0 {
					t.Errorf("violation %d carries no selector, so a report cannot say which pattern is stale", index)
				}
			}

			allowed, err := found.checkable.Check(&kernel.CheckOptions{AllowEmptyTests: true})
			if err != nil {
				t.Fatalf("%s failed with AllowEmptyTests: %v", found.sentence, err)
			}
			if len(allowed) != 0 {
				t.Errorf("%s reports %v with AllowEmptyTests, want the pass: the opt-out does not reach the guard", found.sentence, allowed)
			}
		})
	}
}

// terminal is one Checkable the grammar reaches, with the chain that was typed to get to it — so a failure
// names the sentence a user would have written rather than a reflect.Value.
type terminal struct {
	sentence  string
	checkable kernel.Checkable
}

// terminalsOf are every Checkable reachable from these stages: each stage's predicates, and the object verbs
// chained onto whichever of those are both a terminal and an object stage — `depend on files` is a sentence
// the moment it is typed and still narrows, so it is a terminal at two depths and both are swept.
//
// Two levels is where it stops, because an object verb hands back its own type and a deeper walk would never
// end. A third level would only repeat the same three verbs over a Checkable that is already in the list.
func terminalsOf(t *testing.T, stages ...any) []terminal {
	t.Helper()

	var terminals []terminal
	for _, stage := range stages {
		for _, found := range reachableTerminals(t, "", stage) {
			terminals = append(terminals, found)
			terminals = append(terminals, reachableTerminals(t, found.sentence, found.checkable)...)
		}
	}
	return terminals
}

// reachableTerminals calls every method of one stage that takes the grammar a step further and returns the
// ones that landed on a Checkable: a fluent stage does no work when it is built, so calling all of them is
// free and needs no knowledge of which is which.
//
// A method with anything other than one result is not a step in the chain — `Check` answers with violations
// and an error, `SelectFiles` reads the project — and a result that is not a Checkable is a stage the walk
// leaves to the mood tests. Arguments are synthesized by argumentOf.
func reachableTerminals(t *testing.T, prefix string, stage any) []terminal {
	t.Helper()

	var terminals []terminal
	value := reflect.ValueOf(stage)
	checkable := reflect.TypeOf((*kernel.Checkable)(nil)).Elem()

	for index := range value.NumMethod() {
		method := value.Type().Method(index)
		if method.Type.NumOut() != 1 || !method.Type.Out(0).Implements(checkable) {
			continue
		}

		results := value.Method(index).Call(argumentsFor(t, method))
		found, ok := results[0].Interface().(kernel.Checkable)
		if !ok {
			t.Fatalf("%s returns a %s, which reflect has just said implements Checkable", method.Name, results[0].Type())
		}
		terminals = append(terminals, terminal{
			sentence:  strings.TrimPrefix(prefix+"."+method.Name, "."),
			checkable: found,
		})
	}
	return terminals
}

// argumentsFor is one synthesized argument per parameter the method declares, so that the walk can type a
// chain it knows nothing about. A variadic method is called with none of its variadic arguments, which is
// the chain a user types when they pass only what a verb requires.
func argumentsFor(t *testing.T, method reflect.Method) []reflect.Value {
	t.Helper()

	// The receiver is bound already, so the first parameter of the method's type is not one to synthesize.
	parameters := method.Type.NumIn() - 1
	if method.Type.IsVariadic() {
		parameters--
	}

	arguments := make([]reflect.Value, 0, parameters)
	for index := 1; index <= parameters; index++ {
		arguments = append(arguments, argumentOf(t, method.Type.In(index)))
	}
	return arguments
}

// argumentOf is the one argument of a kind the grammar of this library takes: a pattern, which is the glob
// that matches everything, or a user's own function, which answers whatever its zero value is because the
// guard has to fire before it is ever called.
//
// Anything else is a deliberate failure rather than a zero value, because a verb taking a new kind of
// argument is a verb whose meaning the walk cannot guess — a zero one might quietly turn the rule into a
// sentence nobody meant, and the sweep would then hold the guard to nothing.
func argumentOf(t *testing.T, parameter reflect.Type) reflect.Value {
	t.Helper()

	switch parameter.Kind() {
	case reflect.String:
		// The pattern that selects everything, so that no verb of the chain is the reason nothing matched:
		// the empty selection under test is the scope's stale folder and only that.
		return reflect.ValueOf("**").Convert(parameter)
	case reflect.Func:
		return reflect.MakeFunc(parameter, func([]reflect.Value) []reflect.Value {
			results := make([]reflect.Value, 0, parameter.NumOut())
			for index := range parameter.NumOut() {
				results = append(results, reflect.Zero(parameter.Out(index)))
			}
			return results
		})
	default:
		t.Fatalf("a verb of this module takes a %s, which this walk cannot synthesize: teach argumentOf what a sensible one is", parameter)
		return reflect.Value{}
	}
}
