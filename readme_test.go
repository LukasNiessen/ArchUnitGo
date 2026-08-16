package archunit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// This file is README.md held to the source it describes, by the same argument ci_test.go makes about
// the workflow: a document nobody can fail is a document that goes stale in the commit that changes
// the code. Issue #41 names the failure mode it exists to prevent — a sibling port's README documents
// a slice API that does not exist — and that is not a mistake somebody made while writing it. It is
// what a hand-checked document becomes after a few months of a moving API.
//
// So every Go identifier README.md names is looked up in this module's own syntax tree, every example
// in it is parsed, and the sentences that say a thing is *absent* are checked against the absence. The
// library is read with go/parser rather than reflection, because reflection cannot see a package-level
// function, a method nobody called or a struct field's name, and those are most of what a README says.
//
// Nothing here type-checks the examples. A snippet is written for a reader's project — `internal/api`,
// `docs/architecture.puml`, a predicate over the reader's own files — so the names it uses from this
// library are what can be verified, and they are verified exactly: `archunit.X` against the public
// surface, and every method of a chain rooted at `archunit` against the methods this module declares.

const (
	// readmeFile is the document under test.
	readmeFile = "README.md"
	// surfaceFile is the public surface, and the only file a README example may name a symbol from
	// directly. AGENTS.md: it re-exports and does nothing else.
	surfaceFile = "archunit.go"
	// moduleFile is where the install instruction's module path and Go version have to agree with.
	moduleFile = "go.mod"
	// surfacePackage is the name a README example imports the public surface under. The package is
	// `archunit` while the last element of the module path is `ArchUnitGo`, so every example gives the
	// import that name and every chain in one starts with this identifier.
	surfacePackage = "archunit"
)

// TestTheReadmeExamplesAreValidGo parses every ```go block of the README. An example that does not
// parse is worse than no example: a reader copies it, the compiler complains about the document rather
// than about their code, and they have no way to tell which of the two is wrong.
//
// A block is accepted as a whole file, as a file with the package clause left off — which is how the
// import example showing the ignore directive is written — or as the body of a function, which is how
// every rule in the document is written. Nothing here says which of the three a block has to be,
// because that is the author's business and all three read correctly on the page.
func TestTheReadmeExamplesAreValidGo(t *testing.T) {
	for number, block := range goBlocksOfTheReadme(t) {
		if _, err := parseExample(block); err != nil {
			t.Errorf("the %s Go example in %s does not parse: %v\n%s", ordinal(number+1), readmeFile, err, block)
		}
	}
}

// TestTheReadmeNamesOnlySymbolsThePublicSurfaceHas is the guard against the failure issue #41 names:
// a document describing an API the library does not have.
//
// Every `archunit.X` in an example has to be an exported name of archunit.go, and every method of a
// chain that starts there has to be a method this module declares. That covers the whole of what an
// example can get wrong about this library — a renamed verb, a predicate that moved to the other mood,
// a terminal that never existed — and it costs a parse of the tree the reader is being pointed at.
func TestTheReadmeNamesOnlySymbolsThePublicSurfaceHas(t *testing.T) {
	declared := declarationsOfThisModule(t)

	for number, block := range goBlocksOfTheReadme(t) {
		example, err := parseExample(block)
		if err != nil {
			continue // TestTheReadmeExamplesAreValidGo reports this one.
		}

		for _, selection := range selectionsRootedAtTheSurface(example) {
			if selection.onThePackage && !declared.surface[selection.name] {
				t.Errorf("the %s Go example in %s uses %s.%s, which %s does not export",
					ordinal(number+1), readmeFile, surfacePackage, selection.name, surfaceFile)
			}
			if !selection.onThePackage && !declared.methods[selection.name] {
				t.Errorf("the %s Go example in %s calls .%s() on a chain from %s, and this module declares no such method",
					ordinal(number+1), readmeFile, selection.name, surfacePackage)
			}
		}
		for _, field := range fieldNamesOfTheLiterals(example) {
			if !declared.names[field] {
				t.Errorf("the %s Go example in %s fills in the field %s, which this module declares nowhere",
					ordinal(number+1), readmeFile, field)
			}
		}
	}
}

// TestTheReadmeNamesOnlyIdentifiersThisModuleDeclares is the same guard over the prose, where most of
// a README's names actually are: the grammar table, the verb lists, the fields of an options bag.
//
// Every `Backticked` word that looks like an exported Go identifier has to be something this module
// declares — a function, a type, a constant, a method or a struct field — and a word the document wrote as
// `archunit.X` has to be on the public surface, because that spelling is a promise that a user reaches it
// without importing anything else. A pointer is the type it points at and a qualifier is stepped over, so
// `*CheckOptions` is checked as `CheckOptions` and `CheckOptions.IgnoredImportKinds` as the field. The one
// thing exempted is a segment of the module path, because `ArchUnitGo` is the repository's name and not an
// identifier.
//
// Prose is otherwise checked against the whole module and not against the public surface, because the
// document legitimately names things behind it: the eight LCOM functions it says have no fluent verb yet are
// exactly such names, and a test that demanded they be re-exported would be asking for the opposite of
// what that sentence says.
func TestTheReadmeNamesOnlyIdentifiersThisModuleDeclares(t *testing.T) {
	declared := declarationsOfThisModule(t)
	exempt := append(strings.Split(modulePathOfThisRepository(t), "/"), namesOfTheReadersProject...)

	for _, named := range identifiersTheReadmeProseNames(t) {
		if slices.Contains(exempt, named.name) {
			continue
		}
		switch {
		case named.onTheSurface && !declared.surface[named.name]:
			t.Errorf("%s names `%s`, and %s exports nothing by that name", readmeFile, named.spelled, surfaceFile)
		case !named.onTheSurface && !declared.names[named.name]:
			t.Errorf("%s names `%s` as an identifier of this library, and this module declares nothing by that name",
				readmeFile, named.spelled)
		}
	}
}

// TestTheReadmeInstallsThisModuleTheWayGoModDescribesIt pins the three facts of the install section
// that live in go.mod: the path a user types, the Go version they need, and every dependency they take
// on with it.
//
// The last of those is the one worth a test. "The only direct dependency is golang.org/x/tools" is a
// sentence a reader chooses this library on, and it stops being true in a commit that has nothing to do
// with the README.
func TestTheReadmeInstallsThisModuleTheWayGoModDescribesIt(t *testing.T) {
	readme := readTheReadme(t)
	module := readTheModuleFile(t)

	for _, required := range []string{
		"go get " + modulePathOfThisRepository(t),
		"Go " + valueOfTheModuleDirective(t, "go") + " or newer",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("%s does not say %q, which is what %s says", readmeFile, required, moduleFile)
		}
	}

	for _, dependency := range directRequirementsOfThisRepository(t) {
		if !strings.Contains(readme, dependency) {
			t.Errorf("%s takes a direct dependency on %s and %s does not mention it: the install section says which "+
				"dependencies come with this library", moduleFile, dependency, readmeFile)
		}
	}
	if strings.Contains(module, "// indirect") && !strings.Contains(readme, "direct dependency") {
		t.Errorf("%s does not distinguish direct from indirect dependencies, and %s has both", readmeFile, moduleFile)
	}
}

// TestTheReadmeNamesEveryVerbOfTheClosedSetsItDescribes is the completeness half, for the two families
// whose surface the README states as a closed set rather than as an example.
//
// The metrics family's `Should` verbs are the six threshold predicates and the two zone checks, and the
// README says in as many words that there is no seventh threshold predicate — so a verb added there
// makes that sentence false. The graph family is thirteen terminals and nine modifiers in one type,
// with no mood and no predicate to pass through, so its whole surface is a list a reader picks from and
// a name missing from the document is a name they cannot find at all.
//
// Membership is exact rather than a substring search, and the two counts the sentences state are recomputed
// off the receivers, for the reason TestTheReadmeCountsTheGraphFamilyCorrectly recomputes nine and thirteen:
// `strings.Contains(readme, "ShouldBe")` is satisfied by a document that only names `ShouldBeBelow`, and a
// sentence saying there are six of something is pinned by nothing until somebody counts them.
//
// `String` is exempt from the graph list: the README says once, of every stage of a chain that can describe
// itself, that it is a fmt.Stringer, and repeating it per family would be noise.
func TestTheReadmeNamesEveryVerbOfTheClosedSetsItDescribes(t *testing.T) {
	readme := readTheReadme(t)
	declared := declarationsOfThisModule(t)
	named := namesTheReadmeNames(t)

	for _, verb := range declared.methodsOf("metrics/fluentapi") {
		if !strings.HasPrefix(verb.name, "Should") {
			continue
		}
		if !named[verb.name] {
			t.Errorf("metrics/fluentapi declares the predicate %s and %s does not name it: the README says the "+
				"threshold verbs are a closed set of six, so a seventh has to change that sentence", verb.name, readmeFile)
		}
	}

	for _, verb := range declared.methodsOf("graph/fluentapi") {
		if verb.name == "String" {
			continue
		}
		if !named[verb.name] {
			t.Errorf("graph/fluentapi declares %s and %s does not name it: the report family is one type, so its "+
				"whole surface is the list a reader picks from", verb.name, readmeFile)
		}
	}

	for _, counted := range []struct {
		sentence string
		receiver string
		spelled  int
	}{
		{sentence: "Threshold predicates are exactly six", receiver: "MetricBuilder", spelled: 6},
		{sentence: "the two zone checks", receiver: "MetricsDistanceBuilder", spelled: 2},
	} {
		var found []string
		for _, verb := range declared.methodsOf("metrics/fluentapi") {
			if verb.receiver == counted.receiver && strings.HasPrefix(verb.name, "Should") {
				found = append(found, verb.name)
			}
		}
		if !strings.Contains(readme, counted.sentence) {
			t.Errorf("%s no longer says %q, so nothing here checks the number it states", readmeFile, counted.sentence)
		}
		if len(found) != counted.spelled {
			t.Errorf("%s says %q and %s has %d of them: %v",
				readmeFile, counted.sentence, counted.receiver, len(found), found)
		}
	}
}

// TestTheReadmeCountsTheGraphFamilyCorrectly is the sentence AGENTS.md warns about most — prose that
// counts or bounds a surface — held to the count.
//
// The README says the report family is nine modifiers and thirteen terminals. Both numbers are read off
// the type here rather than trusted: a modifier is a method that hands back another builder, a terminal
// is one that hands back a report or an error, and `Except` is neither because an exclusion qualifies
// the modifier in front of it.
func TestTheReadmeCountsTheGraphFamilyCorrectly(t *testing.T) {
	readme := readTheReadme(t)
	var modifiers, terminals []string

	for _, method := range declarationsOfThisModule(t).methodsOf("graph/fluentapi") {
		switch {
		case strings.HasPrefix(method.name, "Except"):
		case method.results() == "GraphBuilder":
			modifiers = append(modifiers, method.name)
		case strings.HasSuffix(method.results(), "error"):
			terminals = append(terminals, method.name)
		}
	}

	for _, counted := range []struct {
		sentence string
		found    []string
		spelled  int
	}{
		{sentence: "Nine modifiers", found: modifiers, spelled: 9},
		{sentence: "Thirteen terminals", found: terminals, spelled: 13},
	} {
		if !strings.Contains(readme, counted.sentence) {
			t.Errorf("%s no longer says %q, so nothing here checks the number it states", readmeFile, counted.sentence)
		}
		if len(counted.found) != counted.spelled {
			t.Errorf("%s says %q and graph/fluentapi has %d of them: %v",
				readmeFile, counted.sentence, len(counted.found), counted.found)
		}
	}
}

// TestTheReadmeIsHonestAboutWhatIsMissing checks the section a README normally has no way of being
// wrong about safely: the list of what is not implemented yet.
//
// A sentence saying a thing is absent goes stale in the happiest possible way — somebody implements the
// thing — and then the document is telling a reader not to look for a verb that is right there. Each
// row below is one claim of that section, and it fails the day the claim stops being true.
func TestTheReadmeIsHonestAboutWhatIsMissing(t *testing.T) {
	declared := declarationsOfThisModule(t)
	fluentFamilies := []string{
		"files/fluentapi", "layers/fluentapi", "slices/fluentapi", "metrics/fluentapi", "graph/fluentapi",
	}

	for _, family := range fluentFamilies {
		for _, verb := range declared.methodsOf(family) {
			switch {
			case strings.HasPrefix(verb.name, "LCOM"), strings.Contains(verb.name, "Cohesion"):
				t.Errorf("%s declares the verb %s, and %s says the LCOM family has no fluent verb yet",
					family, verb.name, readmeFile)
			case strings.Contains(verb.name, "Package"):
				t.Errorf("%s declares the verb %s, and %s says there are no package-level selectors yet",
					family, verb.name, readmeFile)
			case strings.Contains(verb.name, "FileSuffix"):
				t.Errorf("%s declares the verb %s, and %s says SliceByFileSuffix has no `defined by` verb yet",
					family, verb.name, readmeFile)
			case family == "slices/fluentapi" && strings.HasPrefix(verb.name, "Except"):
				t.Errorf("slices/fluentapi declares %s, and %s says the slices family is the one without `Except`",
					verb.name, readmeFile)
			}
		}
	}

	for name := range declared.surface {
		if strings.Contains(name, "ImportKind") {
			t.Errorf("%s exports %s, and %s says the import kinds are not on the public surface yet",
				surfaceFile, name, readmeFile)
		}
	}
}

// method is one exported method this module declares, as the tests above need it: its name, the type it
// is on, and what it hands back.
type method struct {
	name     string
	receiver string
	returns  []string
}

// results renders what the method hands back as one comma-separated string, so that a test asking
// whether a graph verb is a modifier or a terminal reads as the question it is: `GraphBuilder` is a
// modifier, anything ending in `error` is a terminal.
func (m method) results() string {
	return strings.Join(m.returns, ", ")
}

// declarations is this module's own syntax tree as the tests need to interrogate it: what the public
// surface exports, every exported method name, every exported name of any kind, and the exported
// methods of one package.
type declarations struct {
	// surface is the exported top-level names of archunit.go — the only names an example may write
	// after `archunit.`.
	surface map[string]bool
	// methods is every exported method name in the module, whatever it is on. A chain in an example is
	// checked against this rather than against a receiver, because following a fluent chain's types by
	// hand would be type-checking it, and that is what the compiler is for.
	methods map[string]bool
	// names is every exported name this module declares: functions, types, constants, variables,
	// methods, interface methods and struct fields. It is what the prose is held to.
	names map[string]bool
	// byPackage is the exported methods of each package, keyed by its slash-separated directory.
	byPackage map[string][]method
}

// methodsOf are the exported methods this package declares, in the order they were read.
func (d declarations) methodsOf(pkg string) []method {
	return d.byPackage[pkg]
}

// declarationsOfThisModule reads every non-test Go file of the repository and collects what it declares.
// It is one parse of the tree per test that needs it, which costs a few milliseconds and buys a document
// that cannot name something the library does not have.
func declarationsOfThisModule(t *testing.T) declarations {
	t.Helper()

	declared := declarations{
		surface:   map[string]bool{},
		methods:   map[string]bool{},
		names:     map[string]bool{},
		byPackage: map[string][]method{},
	}

	for _, path := range goFilesOfThisRepository(t) {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s failed: %v", path, err)
		}
		pkg := filepath.ToSlash(filepath.Dir(path))
		for _, declaration := range file.Decls {
			declared.collect(pkg, path, declaration)
		}
	}
	if len(declared.surface) == 0 {
		t.Fatalf("%s declares no exported name at all, so nothing below is checking anything", surfaceFile)
	}
	return declared
}

// collect files one top-level declaration under every name the tests look it up by.
func (d declarations) collect(pkg, path string, declaration ast.Decl) {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if !typed.Name.IsExported() {
			return
		}
		d.names[typed.Name.Name] = true
		if typed.Recv == nil {
			if path == surfaceFile {
				d.surface[typed.Name.Name] = true
			}
			return
		}
		d.methods[typed.Name.Name] = true
		d.byPackage[pkg] = append(d.byPackage[pkg], method{
			name:     typed.Name.Name,
			receiver: strings.TrimPrefix(types.ExprString(typed.Recv.List[0].Type), "*"),
			returns:  resultTypes(typed.Type),
		})
	case *ast.GenDecl:
		for _, spec := range typed.Specs {
			d.collectSpec(path, spec)
		}
	}
}

// collectSpec files a type, constant or variable, and the exported members of a struct or an interface.
// The members are here because most of what a README says about an options bag is its field names.
func (d declarations) collectSpec(path string, spec ast.Spec) {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		d.declare(path, typed.Name)
		switch underlying := typed.Type.(type) {
		case *ast.StructType:
			d.collectFields(underlying.Fields)
		case *ast.InterfaceType:
			d.collectFields(underlying.Methods)
		}
	case *ast.ValueSpec:
		for _, name := range typed.Names {
			d.declare(path, name)
		}
	}
}

// collectFields files the exported members of a struct or an interface. An embedded field has no name
// of its own, so it is skipped: whatever it names is declared where its type is.
func (d declarations) collectFields(fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.IsExported() {
				d.names[name.Name] = true
			}
		}
	}
}

// declare files one exported name, and the public surface's own names twice.
func (d declarations) declare(path string, name *ast.Ident) {
	if !name.IsExported() {
		return
	}
	d.names[name.Name] = true
	if path == surfaceFile {
		d.surface[name.Name] = true
	}
}

// resultTypes are the types a function hands back, as they are written.
func resultTypes(signature *ast.FuncType) []string {
	if signature.Results == nil {
		return nil
	}
	var results []string
	for _, result := range signature.Results.List {
		results = append(results, types.ExprString(result.Type))
	}
	return results
}

// goFilesOfThisRepository are every Go file of the module except the tests, which are not what a README
// describes.
func goFilesOfThisRepository(t *testing.T) []string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking this repository failed: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("this repository holds no Go file outside its tests")
	}
	return paths
}

// selection is one thing a README example reads off the public surface: a name after `archunit.`, or a
// method of a chain that starts there.
type selection struct {
	name         string
	onThePackage bool
}

// selectionsRootedAtTheSurface are every name an example reads off this library, and nothing else.
//
// A selector expression is followed back to what it is rooted at — through calls, indexes and the
// selectors before it — and it counts only when that root is the identifier the public surface is
// imported under. That is what keeps `fmt.Println(snapshot.Summary())` out of the result while
// `archunit.ProjectFiles(nil).InFolder("x")` is fully in it: an example is checked against this library
// and never against the standard one.
func selectionsRootedAtTheSurface(example *ast.File) []selection {
	var found []selection

	ast.Inspect(example, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		root, isIdentifier := chainRoot(selector.X).(*ast.Ident)
		if !isIdentifier || root.Name != surfacePackage {
			return true
		}
		qualifier, onThePackage := selector.X.(*ast.Ident)
		found = append(found, selection{
			name:         selector.Sel.Name,
			onThePackage: onThePackage && qualifier.Name == surfacePackage,
		})
		return true
	})
	return found
}

// chainRoot is what an expression is ultimately read off: the identifier at the far left of a fluent
// chain, once every call, selector, index and pair of parentheses in front of it has been stepped over.
func chainRoot(expression ast.Expr) ast.Expr {
	for {
		switch node := expression.(type) {
		case *ast.SelectorExpr:
			expression = node.X
		case *ast.CallExpr:
			expression = node.Fun
		case *ast.IndexExpr:
			expression = node.X
		case *ast.ParenExpr:
			expression = node.X
		default:
			return expression
		}
	}
}

// fieldNamesOfTheLiterals are the fields an example fills in on a composite literal — the knobs of an
// options bag, which is where a README is as likely to name something stale as in a chain.
func fieldNamesOfTheLiterals(example *ast.File) []string {
	var found []string

	ast.Inspect(example, func(node ast.Node) bool {
		literal, isLiteral := node.(*ast.CompositeLit)
		if !isLiteral {
			return true
		}
		for _, element := range literal.Elts {
			pair, isPair := element.(*ast.KeyValueExpr)
			if !isPair {
				continue
			}
			if key, isIdentifier := pair.Key.(*ast.Ident); isIdentifier && key.IsExported() {
				found = append(found, key.Name)
			}
		}
		return true
	})
	return found
}

// namesOfTheReadersProject are the backticked capitalised words of the README that name something in the
// project a reader is holding this library against, not something in the library. `Handler` is the
// declared type the match-target table walks `internal/api/handler.go` through, and a document that
// could not name one would have to explain `ForClassesMatching` without an example of a class.
//
// The list is deliberately short. Anything on it is a name nothing checks, which is the cost of writing
// an example, so a new entry is a decision rather than a way around a failing test.
var namesOfTheReadersProject = []string{"Handler"}

// namesTheReadmeQualifiesWithSomethingElse are the backticked words of the README that read as a Go name
// qualified by something this module is not: an interface of the standard library, a package behind the
// public surface written as a user would have to import it, and a declared type of the reader's own project.
//
// They are exempt for the same reason namesOfTheReadersProject is, and the list is short for the same
// reason: each entry is a name nothing checks.
var namesTheReadmeQualifiesWithSomethingElse = []string{
	"fmt.Stringer", "extraction.ImportKindSet", "internal/api.Handler",
}

// identifierInProse is a backticked word in the README that reads as an exported Go identifier: one
// word, starting with a capital. A pattern, a shell command, a path and a lowercase keyword all fail it,
// which is the point — those are not names this module can be asked about.
var identifierInProse = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// proseName is one backticked word of the README that reads as a Go name, and where it has to be looked up.
type proseName struct {
	// spelled is the word as the document wrote it, qualifier and pointer included, so that a failure
	// quotes the reader what they would search the document for.
	spelled string
	// name is the identifier itself: the word with a leading `*` and any qualifier taken off.
	name string
	// onTheSurface says the document wrote the name as `archunit.X`, which is a promise that archunit.go
	// exports it and not merely that this module declares it somewhere.
	onTheSurface bool
}

// identifiersTheReadmeProseNames are the identifiers the document names outside its code blocks: every
// `Backticked` word that reads as an exported Go name, with a trailing `()` taken off so that `Kind()`
// is checked as the method it is.
//
// A pointer and a qualifier are read through rather than discarded, because the names that appear *only* in
// the prose are mostly written one of those two ways — `archunit.KindEmptyTest`, `*ProjectLocator`,
// `CheckOptions.IgnoredImportKinds` — and a guard that dropped every shape but the bare word would be
// checking nothing about exactly the names no example mentions. A qualifier this module cannot be asked
// about ends the word instead: `AGENTS.md`, `go.mod` and `t.Error` are not names of this library, and are
// told apart by the qualifier reading as an exported identifier and the member after it doing so too.
//
// The code blocks are cut out first because they are checked properly, against the syntax tree, by the
// two tests above.
func identifiersTheReadmeProseNames(t *testing.T) []proseName {
	t.Helper()

	var found []proseName
	for _, quoted := range regexp.MustCompile("`([^`\n]+)`").FindAllStringSubmatch(proseOfTheReadme(t), -1) {
		spelled := strings.TrimSuffix(quoted[1], "()")
		if slices.Contains(namesTheReadmeQualifiesWithSomethingElse, spelled) {
			continue
		}
		qualifier, name, qualified := strings.Cut(strings.TrimPrefix(spelled, "*"), ".")
		if !qualified {
			qualifier, name = "", qualifier
		}
		if !identifierInProse.MatchString(name) {
			continue
		}
		if qualified && qualifier != surfacePackage && !identifierInProse.MatchString(qualifier) {
			continue
		}
		found = append(found, proseName{spelled: spelled, name: name, onTheSurface: qualifier == surfacePackage})
	}
	if len(found) == 0 {
		t.Fatalf("%s names no identifier of this library outside its code blocks", readmeFile)
	}
	return found
}

// namesTheReadmeNames is every name the document states as a name, as a set: the identifiers of its prose
// and the selections of its examples.
//
// It is what a completeness check has to be written against rather than a substring search over the
// document, because a substring search cannot fail for a prefix — a README that names `ShouldBeBelow` and
// nothing else satisfies `strings.Contains(readme, "ShouldBe")`, so three of the six threshold verbs would
// be pinned by nothing.
func namesTheReadmeNames(t *testing.T) map[string]bool {
	t.Helper()

	named := map[string]bool{}
	for _, name := range identifiersTheReadmeProseNames(t) {
		named[name.name] = true
	}
	for _, block := range goBlocksOfTheReadme(t) {
		example, err := parseExample(block)
		if err != nil {
			continue // TestTheReadmeExamplesAreValidGo reports this one.
		}
		for _, selection := range selectionsRootedAtTheSurface(example) {
			named[selection.name] = true
		}
	}
	return named
}

// proseOfTheReadme is the document with its fenced code blocks removed.
func proseOfTheReadme(t *testing.T) string {
	t.Helper()

	var prose []string
	fenced := false
	for line := range strings.Lines(readTheReadme(t)) {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if !fenced {
			prose = append(prose, line)
		}
	}
	return strings.Join(prose, "")
}

// goBlocksOfTheReadme are the document's ```go blocks, in the order they appear. A block fenced as
// anything else — the shell command, the failure message a rule prints, the layout table — is not Go
// and is not read as Go.
func goBlocksOfTheReadme(t *testing.T) []string {
	t.Helper()

	var blocks []string
	var current []string
	fenced := false
	for line := range strings.Lines(readTheReadme(t)) {
		switch trimmed := strings.TrimSpace(line); {
		case fenced && strings.HasPrefix(trimmed, "```"):
			blocks = append(blocks, strings.Join(current, ""))
			current, fenced = nil, false
		case fenced:
			current = append(current, line)
		case strings.HasPrefix(trimmed, "```go"):
			fenced = true
		}
	}
	if fenced {
		t.Fatalf("%s has a code fence that is never closed", readmeFile)
	}
	if len(blocks) == 0 {
		t.Fatalf("%s holds no Go example at all", readmeFile)
	}
	return blocks
}

// parseExample parses one README block the three ways a block is legitimately written: a whole file, a
// file with the package clause left off, or the body of a function. The first spelling that parses wins,
// and the error reported when none of them does is the one from reading it as a function body, because
// that is what nearly every example in the document is.
func parseExample(block string) (*ast.File, error) {
	if strings.HasPrefix(block, "package ") {
		return parser.ParseFile(token.NewFileSet(), "example.go", block, parser.SkipObjectResolution)
	}
	if file, err := parser.ParseFile(
		token.NewFileSet(), "example.go", "package example\n"+block, parser.SkipObjectResolution,
	); err == nil {
		return file, nil
	}
	return parser.ParseFile(
		token.NewFileSet(), "example.go",
		"package example\nfunc example() {\n"+block+"\n}\n", parser.SkipObjectResolution,
	)
}

// readTheReadme returns the document's contents, failing the test if the one document a user starts from
// is not there.
func readTheReadme(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("reading %s failed: %v", readmeFile, err)
	}
	return string(content)
}

// readTheModuleFile returns go.mod's contents.
func readTheModuleFile(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(moduleFile)
	if err != nil {
		t.Fatalf("reading %s failed: %v", moduleFile, err)
	}
	return string(content)
}

// modulePathOfThisRepository is what a user types after `go get`, read off go.mod rather than spelled
// here: an install instruction naming a path this module does not have is the one mistake in a README
// that stops a reader at the first line.
func modulePathOfThisRepository(t *testing.T) string {
	t.Helper()

	return valueOfTheModuleDirective(t, "module")
}

// valueOfTheModuleDirective is the single-argument directive go.mod states under this keyword —
// `module` and `go` are the two the README quotes.
func valueOfTheModuleDirective(t *testing.T, keyword string) string {
	t.Helper()

	for line := range strings.Lines(readTheModuleFile(t)) {
		if value, stated := strings.CutPrefix(strings.TrimSpace(line), keyword+" "); stated {
			return strings.TrimSpace(value)
		}
	}
	t.Fatalf("%s states no %s directive", moduleFile, keyword)
	return ""
}

// directRequirementsOfThisRepository are the modules this one requires itself, as opposed to the ones
// its own dependencies pulled in. They are what the install section promises a reader they are taking on.
func directRequirementsOfThisRepository(t *testing.T) []string {
	t.Helper()

	var required []string
	for line := range strings.Lines(readTheModuleFile(t)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "// indirect") || !strings.Contains(trimmed, ".") {
			continue
		}
		if path, version, versioned := strings.Cut(trimmed, " v"); versioned && version != "" {
			required = append(required, strings.TrimPrefix(path, "require "))
		}
	}
	if len(required) == 0 {
		t.Fatalf("%s requires no module directly, and the install section says which dependencies come along",
			moduleFile)
	}
	return required
}

// ordinal names a code block the way the test's own message needs it, so a failure says which example of
// the document it is about rather than printing a line number nobody can find in a rendered README.
func ordinal(number int) string {
	names := []string{
		"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth",
		"eleventh", "twelfth", "thirteenth", "fourteenth", "fifteenth", "sixteenth", "seventeenth",
		"eighteenth", "nineteenth", "twentieth",
	}
	if number >= 1 && number <= len(names) {
		return names[number-1]
	}
	return "#" + strconv.Itoa(number)
}
