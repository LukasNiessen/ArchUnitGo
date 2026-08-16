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

	// The same guard held to the third level of the walk, which the list above cannot see: every name in it
	// is a predicate the first level finds, so a third level that found nothing — or one whose `except`
	// verbs were folded back into the second — would leave the sweep saying nothing about the companion an
	// exclusion is. An exclusion is only valid after a verb that selected something, so each of them has to
	// be reachable through an object verb and nowhere else.
	for _, sentence := range []string{
		"DependOnFiles.InFolder.Except",
		"DependOnFiles.InFolder.ExceptWithName",
		"DependOnFiles.InFolder.ExceptInFolder",
		"DependOnFiles.InFolder.ExceptInPath",
		"DependOnExternalModules.Matching.Except",
	} {
		if !slices.ContainsFunc(terminals, func(found terminal) bool { return found.sentence == sentence }) {
			t.Fatalf("the walk found %d terminals and none of them is %s: it has gone blind, and the sweep below proves nothing", len(terminals), sentence)
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

// terminalsOf are every Checkable reachable from these stages: each stage's predicates, the object verbs
// chained onto whichever of those are both a terminal and an object stage — `depend on files` is a sentence
// the moment it is typed and still narrows, so it is a terminal at two depths and both are swept — and the
// `except` companion of each of those object verbs.
//
// Three levels is where it stops, and the third exists for one reason: an exclusion qualifies the selector
// it follows, so `depend on files, except "**"` is a misuse the fluent API rejects rather than a step in the
// chain, and the only valid place to type an exclusion is after a verb that selected something. Deeper than
// that an object verb hands back its own type and the walk would never end, while repeating the same verbs
// over a Checkable already in the list proves nothing new.
func terminalsOf(t *testing.T, stages ...any) []terminal {
	t.Helper()

	var terminals []terminal
	for _, stage := range stages {
		for _, predicate := range reachableTerminals(t, "", stage, narrowing) {
			terminals = append(terminals, predicate)
			for _, object := range reachableTerminals(t, predicate.sentence, predicate.checkable, narrowing) {
				terminals = append(terminals, object)
				terminals = append(terminals, reachableTerminals(t, object.sentence, object.checkable, excluding)...)
			}
		}
	}
	return terminals
}

// narrowing and excluding are the two halves of a stage's method set: the verbs that select, and the `except`
// companions that qualify what a verb selected. The walk needs them apart because the order they are typed in
// is part of the grammar — everything else about a stage it discovers by reflection.
func narrowing(method string) bool {
	return !excluding(method)
}

func excluding(method string) bool {
	return strings.HasPrefix(method, "Except")
}

// reachableTerminals calls every method of one stage that takes the grammar a step further and returns the
// ones that landed on a Checkable: a fluent stage does no work when it is built, so calling all of them is
// free and needs no knowledge of which is which.
//
// A method with anything other than one result is not a step in the chain — `Check` answers with violations
// and an error, `SelectFiles` reads the project — and a result that is not a Checkable is a stage the walk
// leaves to the mood tests. wanted is which half of the method set this level is after, as terminalsOf
// describes. Arguments are synthesized by argumentOf.
func reachableTerminals(t *testing.T, prefix string, stage any, wanted func(string) bool) []terminal {
	t.Helper()

	var terminals []terminal
	value := reflect.ValueOf(stage)
	checkable := reflect.TypeOf((*kernel.Checkable)(nil)).Elem()

	for index := range value.NumMethod() {
		method := value.Type().Method(index)
		if method.Type.NumOut() != 1 || !method.Type.Out(0).Implements(checkable) || !wanted(method.Name) {
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
// chain it knows nothing about. A variadic method is called with exactly one variadic argument, because the
// verbs of this module that take a variadic list — the `except` companions — require it: a verb the user typed
// that narrows nothing is reported rather than ignored, and that rejection is not what this sweep is about.
func argumentsFor(t *testing.T, method reflect.Method) []reflect.Value {
	t.Helper()

	// The receiver is bound already, so the first parameter of the method's type is not one to synthesize.
	parameters := method.Type.NumIn() - 1
	if method.Type.IsVariadic() {
		parameters--
	}

	pattern := selectsEverything
	if excluding(method.Name) {
		pattern = excludesNothing
	}

	arguments := make([]reflect.Value, 0, parameters+1)
	for index := 1; index <= parameters; index++ {
		arguments = append(arguments, argumentOf(t, method.Type.In(index), pattern))
	}
	if method.Type.IsVariadic() {
		// reflect.Value.Call takes the variadic arguments spread, so the element type is what to synthesize.
		arguments = append(arguments, argumentOf(t, method.Type.In(method.Type.NumIn()-1).Elem(), pattern))
	}
	return arguments
}

// The two patterns the walk types, and both are there so that no verb it synthesized is the reason nothing
// matched: the empty selection under test is the scope's stale folder and only that.
const (
	// selectsEverything is what a selecting verb is given.
	selectsEverything = "**"
	// excludesNothing is what an `except` companion is given: a pattern no identifier can match, because an
	// exclusion narrows from the other side and `**` there would empty the selection the verb before it made.
	excludesNothing = "not the name of any file in any project"
)

// argumentOf is the one argument of a kind the grammar of this library takes: pattern, which is the one the
// verb being called should be given, or a user's own function, which answers whatever its zero value is
// because the guard has to fire before it is ever called.
//
// Anything else is a deliberate failure rather than a zero value, because a verb taking a new kind of
// argument is a verb whose meaning the walk cannot guess — a zero one might quietly turn the rule into a
// sentence nobody meant, and the sweep would then hold the guard to nothing.
func argumentOf(t *testing.T, parameter reflect.Type, pattern string) reflect.Value {
	t.Helper()

	switch parameter.Kind() {
	case reflect.String:
		return reflect.ValueOf(pattern).Convert(parameter)
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
