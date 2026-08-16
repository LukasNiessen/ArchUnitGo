package archunit_test

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// This file is the documentation site under docs/ held to the library it documents, by the same argument
// readme_test.go makes about README.md: a document nobody can fail is a document that goes stale in the
// commit that changes the code. A site is the easiest place in a repository for that to happen quietly,
// because it is ten pages nobody reads in a diff and it renders just as nicely when it is wrong.
//
// So every page is read the same way the README is — its examples parsed, its `archunit.X` names looked up
// on the public surface, every backticked word of its prose looked up in this module's own syntax tree —
// through the helpers in readme_test.go rather than through a second copy of them. What is new here is what
// only a site can get wrong: a page with no front matter is a page the navigation cannot show, a link to a
// page nobody wrote is a 404, and a family page missing a verb is a verb a reader cannot find at all.
//
// The completeness half is the reason the site is worth having a test for rather than a proofread. Nine
// pages of prose beside a moving fluent API is exactly the situation issue #41 names, and the check that
// scales is not "did somebody remember to document this" — it is "every exported verb of files/fluentapi
// appears on docs/files.md, or this build is red".

const (
	// docsFolder is the site: every Markdown file directly in it is a page, and the two folders beside
	// them are the layout and the stylesheet. The paths below are written with forward slashes because
	// that is what a Markdown link between two pages says as well as a path os.ReadFile takes anywhere.
	docsFolder = "docs"
	// docsLandingPage is the page a reader arrives at, and the one that links to all the others.
	docsLandingPage = "docs/index.md"
	// docsGrammarPage is where the vocabulary shared by every family is documented once, which is what
	// lets the family pages leave the three self-describing methods out — and where the entry points and
	// each family's exclusion verbs are stated as counted sets nobody else's page repeats.
	docsGrammarPage = "docs/grammar.md"
	// docsPatternsPage is the glob syntax, and with it the projections behind the slicing verbs: a table of
	// the ones a caller can hold, stated as a count of what the surface exports.
	docsPatternsPage = "docs/patterns.md"
	// docsMetricsPage and docsGraphPage state almost the whole of their surface as a count, the way the
	// README does — and a sentence saying there are six of something is pinned by nothing until somebody
	// counts them.
	docsMetricsPage = "docs/metrics.md"
	docsGraphPage   = "docs/graph.md"
	// docsRunningPage and docsSlicesPage state a surface as the names themselves: the violation kinds, the
	// colors, the log levels, the ways a project and a diagram of it disagree. A list of twelve is complete
	// until a thirteenth constant is exported, and stale from that commit on.
	docsRunningPage = "docs/running.md"
	docsSlicesPage  = "docs/slices.md"
	// docsConfigFile, docsLayoutFile and docsStylesheet are the whole of the site's machinery. There is
	// no generator, no lockfile and no build step: GitHub's own Jekyll pours the pages into the layout.
	docsConfigFile = "docs/_config.yml"
	docsLayoutFile = "docs/_layouts/default.html"
	docsStylesheet = "docs/assets/docs.css"
	// pagesWorkflow publishes the site and checks nothing, which is what the last test below is about.
	pagesWorkflow = ".github/workflows/pages.yml"
)

// TestTheDocsSiteExamplesAreValidGo parses every ```go block of every page, for the reason the README's
// examples are parsed: a reader copies one, the compiler complains about the document rather than about
// their code, and they have no way to tell which of the two is wrong.
func TestTheDocsSiteExamplesAreValidGo(t *testing.T) {
	examples := 0

	for _, documented := range pagesOfTheDocsSite(t) {
		for number, block := range goBlocksOf(t, documented.path) {
			examples++
			if _, err := parseExample(block); err != nil {
				t.Errorf("the %s Go example in %s does not parse: %v\n%s",
					ordinal(number+1), documented.path, err, block)
			}
		}
	}
	if examples == 0 {
		t.Errorf("%s holds no Go example at all, so nothing here is checking anything about this library",
			docsFolder)
	}
}

// TestTheDocsSiteNamesOnlySymbolsThePublicSurfaceHas is the guard against a site describing an API the
// library does not have: every `archunit.X` in an example is an exported name of archunit.go, every method
// of a chain that starts there is a method this module declares, and every field an example fills in on an
// options bag is a field that exists.
func TestTheDocsSiteNamesOnlySymbolsThePublicSurfaceHas(t *testing.T) {
	declared := declarationsOfThisModule(t)

	for _, documented := range pagesOfTheDocsSite(t) {
		for number, block := range goBlocksOf(t, documented.path) {
			example, err := parseExample(block)
			if err != nil {
				continue // TestTheDocsSiteExamplesAreValidGo reports this one.
			}

			for _, used := range selectionsRootedAtTheSurface(example) {
				if used.onThePackage && !declared.surface[used.name] {
					t.Errorf("the %s Go example in %s uses %s.%s, which %s does not export",
						ordinal(number+1), documented.path, surfacePackage, used.name, surfaceFile)
				}
				if !used.onThePackage && !declared.methods[used.name] {
					t.Errorf("the %s Go example in %s calls .%s() on a chain from %s, and this module declares "+
						"no such method", ordinal(number+1), documented.path, used.name, surfacePackage)
				}
			}
			for _, field := range fieldNamesOfTheLiterals(example) {
				if !declared.names[field] {
					t.Errorf("the %s Go example in %s fills in the field %s, which this module declares nowhere",
						ordinal(number+1), documented.path, field)
				}
			}
		}
	}
}

// TestTheDocsSiteNamesOnlyIdentifiersThisModuleDeclares is the same guard over the prose, where most of a
// documentation page's names are: the tables of verbs, the fields of a struct a predicate is handed, the
// sentinel errors a terminal returns.
//
// It is the same check as the README's, against the same two short exemption lists, because the two
// documents describe one library and a name that is fine in one is fine in the other.
func TestTheDocsSiteNamesOnlyIdentifiersThisModuleDeclares(t *testing.T) {
	declared := declarationsOfThisModule(t)
	exempt := append(strings.Split(modulePathOfThisRepository(t), "/"), namesOfTheReadersProject...)

	for _, documented := range pagesOfTheDocsSite(t) {
		inTheProse := identifiersNamedInTheProseOf(t, documented.path)
		if len(inTheProse) == 0 {
			t.Errorf("%s names no identifier of this library outside its code blocks: a page of this site that "+
				"is only prose is a page nothing checks", documented.path)
		}

		for _, named := range inTheProse {
			if slices.Contains(exempt, named.name) {
				continue
			}
			switch {
			case named.onTheSurface && !declared.surface[named.name]:
				t.Errorf("%s names `%s`, and %s exports nothing by that name",
					documented.path, named.spelled, surfaceFile)
			case !named.onTheSurface && !declared.names[named.name]:
				t.Errorf("%s names `%s` as an identifier of this library, and this module declares nothing by "+
					"that name", documented.path, named.spelled)
			}
		}
	}
}

// TestEveryPageOfTheDocsSiteSaysWhereItBelongs pins the front matter, which is the only thing holding the
// site together: the layout builds its navigation out of every page that gives itself a nav_order, so a
// page without one is a page nobody can reach, and two pages with the same one are ordered by whatever
// Jekyll happened to sort them by.
//
// The orders are 1..N with no gap and no repeat, and the title in the front matter is the page's own first
// heading — otherwise the navigation and the page disagree about what the page is called.
func TestEveryPageOfTheDocsSiteSaysWhereItBelongs(t *testing.T) {
	pages := pagesOfTheDocsSite(t)
	atPosition := map[int]string{}

	for _, documented := range pages {
		if layout := documented.frontMatter["layout"]; layout != "default" {
			t.Errorf("%s states the layout %q: every page of this site is poured into %s, which is where its "+
				"navigation and its stylesheet come from", documented.path, layout, docsLayoutFile)
		}
		for _, key := range []string{"title", "description"} {
			if documented.frontMatter[key] == "" {
				t.Errorf("%s states no %s in its front matter: the navigation is built from the title and the "+
					"description is what a search result and a link preview show", documented.path, key)
			}
		}
		if documented.heading != documented.frontMatter["title"] {
			t.Errorf("%s is called %q in its front matter and %q in its own first heading: the navigation and "+
				"the page would disagree about the name of the page",
				documented.path, documented.frontMatter["title"], documented.heading)
		}

		order, stated := navOrderOf(documented)
		switch {
		case !stated:
			t.Errorf("%s states no nav_order: %s builds the navigation from that key alone, so a page without "+
				"one is published and linked from nowhere", documented.path, docsLayoutFile)
		case atPosition[order] != "":
			t.Errorf("%s and %s both state nav_order %d, so the navigation puts them in whichever order they "+
				"happened to be sorted in", atPosition[order], documented.path, order)
		default:
			atPosition[order] = documented.path
		}
	}

	for position := 1; position <= len(pages); position++ {
		if atPosition[position] == "" {
			t.Errorf("%s holds %d pages and none of them states nav_order %d: the orders are 1..%d, so a gap "+
				"means a page gave itself an order the site has no room for",
				docsFolder, len(pages), position, len(pages))
		}
	}
	if atPosition[1] != docsLandingPage {
		t.Errorf("%s is the first chapter of the navigation and %s is the page a reader arrives at",
			atPosition[1], docsLandingPage)
	}
}

// TestEveryLinkOfTheDocsSiteResolves is the check a rendered site cannot do for itself. These pages link to
// each other by filename — `[the files family](files.md)` — so that every link works while reading the
// source on GitHub as well as on the built site, where jekyll-relative-links rewrites them. The cost of
// that spelling is that a link to a page nobody wrote looks exactly like a link to one that exists.
//
// A fragment is resolved too, against the target page's own headings, because a renamed heading is the way
// a link between two pages half-breaks: it lands on the page and then somewhere arbitrary on it.
func TestEveryLinkOfTheDocsSiteResolves(t *testing.T) {
	for _, documented := range pagesOfTheDocsSite(t) {
		targets := linkTargetsOf(t, documented.path)
		if len(targets) == 0 {
			t.Errorf("%s links to nothing at all, not even to the page after it: this site is read in order",
				documented.path)
		}

		for _, target := range targets {
			if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue // an anchor inside this page, or an address; neither is a file of this site.
			}
			if strings.HasPrefix(target, "http") {
				if !strings.HasPrefix(target, "https://") {
					t.Errorf("%s links to %s over plain http", documented.path, target)
				}
				continue // somebody else's server, which nothing in this repository can hold to existing.
			}

			file, fragment, _ := strings.Cut(target, "#")
			if !strings.HasSuffix(file, ".md") {
				t.Errorf("%s links to %s, and an internal link of this site is a page's filename: that is what "+
					"works in the repository as well as on the built site", documented.path, target)
				continue
			}
			if _, err := os.Stat(docsFolder + "/" + file); err != nil {
				t.Errorf("%s links to %s, and %s holds no such page", documented.path, target, docsFolder)
				continue
			}
			if fragment == "" {
				continue
			}
			if !slices.ContainsFunc(headingsOf(t, docsFolder+"/"+file), func(heading string) bool {
				return anchorOf(heading) == fragment
			}) {
				t.Errorf("%s links to %s, and %s has no heading that lands on #%s",
					documented.path, target, file, fragment)
			}
		}
	}
}

// TestTheDocsSiteLinksInMarkdownRatherThanInHtml is the one rule about how a page is written, and it is
// here because breaking it produces a site whose links work in the repository and 404 once published —
// the one kind of broken link nobody notices.
//
// jekyll-relative-links rewrites Markdown links only. A raw <a href="files.md"> is left exactly as it was
// written, and a page served at /ArchUnitGo/files/ then points at /ArchUnitGo/files/files.md.
func TestTheDocsSiteLinksInMarkdownRatherThanInHtml(t *testing.T) {
	for _, documented := range pagesOfTheDocsSite(t) {
		content := readDocument(t, documented.path)
		for _, raw := range []string{"href=", "<a "} {
			if strings.Contains(content, raw) {
				t.Errorf("%s writes a link as raw HTML (%q): only a Markdown link is rewritten for the built "+
					"site, so this one works in the repository and 404s once published", documented.path, raw)
			}
		}
	}
}

// TestEveryPageOfTheDocsSiteIsLinkedFromTheLandingPage is the guard against the orphan page — written,
// published, in the navigation of a site nobody navigates by the navigation, and never linked from the one
// page every reader starts on.
func TestEveryPageOfTheDocsSiteIsLinkedFromTheLandingPage(t *testing.T) {
	linked := map[string]bool{}
	for _, target := range linkTargetsOf(t, docsLandingPage) {
		file, _, _ := strings.Cut(target, "#")
		linked[file] = true
	}

	for _, documented := range pagesOfTheDocsSite(t) {
		if documented.path == docsLandingPage || linked[documented.name] {
			continue
		}
		t.Errorf("%s does not link to %s: a page reachable only from the navigation is a page a reader who "+
			"followed the site in order never sees", docsLandingPage, documented.name)
	}
}

// TestEveryFamilyPageNamesEveryVerbOfItsFamily is the completeness check, and the reason this site is
// worth writing at all: the five fluent packages are the whole product, and each has a page whose job is
// to name every verb it offers. A verb documented nowhere is a verb a reader cannot find — they do not
// know it exists, so they cannot search pkg.go.dev for it either.
//
// Membership is exact rather than a substring search, for the reason namesTheDocumentNames exists: a page
// naming `ShouldBeBelow` and nothing else satisfies strings.Contains(page, "ShouldBe").
//
// The three self-describing methods are exempt because docs/grammar.md documents them once for every
// family — which is checked here too, so the exemption stays honest instead of becoming three verbs no
// page has to name.
func TestEveryFamilyPageNamesEveryVerbOfItsFamily(t *testing.T) {
	declared := declarationsOfThisModule(t)

	grammar := namesTheDocumentNames(t, docsGrammarPage)
	for _, shared := range methodsEveryFamilyShares() {
		if !grammar[shared] {
			t.Errorf("%s does not name %s, and every family page is excused from naming it because that page "+
				"documents it once", docsGrammarPage, shared)
		}
	}

	for _, family := range []string{"files", "layers", "slices", "metrics", "graph"} {
		page := docsFolder + "/" + family + ".md"
		fluent := family + "/fluentapi"

		verbs := declared.methodsOf(fluent)
		if len(verbs) == 0 {
			t.Fatalf("%s declares no exported method, so nothing here is checking %s", fluent, page)
		}

		named := namesTheDocumentNames(t, page)
		for _, verb := range verbs {
			if slices.Contains(methodsEveryFamilyShares(), verb.name) || named[verb.name] {
				continue
			}
			t.Errorf("%s declares the verb %s and %s does not name it: a verb missing from its family's page "+
				"is a verb a reader has no way of knowing exists", fluent, verb.name, page)
		}
	}
}

// TestTheDocsSiteCountsTheSurfacesItStatesAsClosedSets holds the sentences AGENTS.md warns about most —
// prose that counts or bounds a surface — to the count.
//
// Every number below is read off the syntax tree rather than trusted: a modifier is a method that hands
// back its own builder, a terminal is one that hands something back and can fail, a metric verb hands back
// the stage that can be resolved, and an exclusion is counted as its own set because it qualifies the verb
// in front of it rather than being one a reader picks from a list. A verb added to any of these groups makes
// a sentence false, and that is the day this test is useful.
//
// A set stated as the names themselves is the other half of it, and the half a count cannot catch: the page
// listing the twelve violation kinds is complete until a thirteenth is exported, and nothing about the
// sentence changes on the day one is. So every member of those sets is read off the surface too, and looked
// up on the page that enumerates it.
//
// The last half is a set stated as a name that is *not* there. The entry-point table gives two families a
// dash in its `Also spelled` column, and neither a count of the surface nor a lookup of what a page names can
// fail for a spelling somebody added — so those two are checked against the absence, the way
// TestTheReadmeIsHonestAboutWhatIsMissing checks the README's.
func TestTheDocsSiteCountsTheSurfacesItStatesAsClosedSets(t *testing.T) {
	declared := declarationsOfThisModule(t)
	const graphFluent, metricsFluent = "graph/fluentapi", "metrics/fluentapi"
	const filesFluent, layersFluent = "files/fluentapi", "layers/fluentapi"

	comparisons := verbsOf(declared, metricsFluent, "MetricBuilder", "MetricsThresholdCondition")

	for _, counted := range []struct {
		page     string
		sentence string
		found    []string
		spelled  int
	}{
		{
			page: docsGraphPage, sentence: "The nine modifiers", spelled: 9,
			found: verbsOf(declared, graphFluent, "GraphBuilder", "GraphBuilder"),
		},
		{
			page: docsGraphPage, sentence: "The thirteen terminals", spelled: 13,
			found: terminalsOf(declared, graphFluent, "GraphBuilder"),
		},
		{
			page: docsMetricsPage, sentence: "Four scope verbs", spelled: 4,
			found: verbsOf(declared, metricsFluent, "MetricsBuilder", "MetricsBuilder"),
		},
		{
			page: docsMetricsPage, sentence: "is the eight numbers", spelled: 8,
			found: verbsOf(declared, metricsFluent, "MetricsCountBuilder", "MetricBuilder"),
		},
		{
			page: docsMetricsPage, sentence: "There are exactly six threshold predicates", spelled: 6,
			found: slices.Concat(
				comparisons,
				verbsOf(declared, metricsFluent, "MetricBuilder", "MetricsSatisfactionCondition"),
			),
		},
		{
			page: docsMetricsPage, sentence: "Five hold every number a rule measured to a figure",
			found: comparisons, spelled: 5,
		},
		{
			page: docsMetricsPage, sentence: "The two zones", spelled: 2,
			found: verbsOf(declared, metricsFluent, "MetricsDistanceBuilder", "MetricsZoneCondition"),
		},
		{
			page: docsGrammarPage, sentence: "four in the files family", spelled: 4,
			found: exclusionsOf(declared, filesFluent),
		},
		{
			page: docsGrammarPage, sentence: "five in metrics", spelled: 5,
			found: exclusionsOf(declared, metricsFluent),
		},
		{
			page: docsGrammarPage, sentence: "three in layers", spelled: 3,
			found: exclusionsOf(declared, layersFluent),
		},
	} {
		if !strings.Contains(readDocument(t, counted.page), counted.sentence) {
			t.Errorf("%s no longer says %q, so nothing here checks the number it states",
				counted.page, counted.sentence)
		}
		if len(counted.found) != counted.spelled {
			t.Errorf("%s says %q and this library has %d of them: %v",
				counted.page, counted.sentence, len(counted.found), counted.found)
		}
	}

	for _, enumerated := range []struct {
		page     string
		sentence string
		members  []string
		// spelled is the number the sentence says out loud, or 0 where it states only the names.
		spelled int
	}{
		{
			page: docsRunningPage, sentence: "The kinds are",
			members: membersOfTheSurfacePrefixed(declared, "Kind"),
		},
		{
			page: docsRunningPage, sentence: "`Color` is a closed set",
			members: membersOfTheSurfacePrefixed(declared, "Color"),
		},
		{
			page: docsRunningPage, sentence: "the four levels are", spelled: 4,
			members: membersOfTheSurfacePrefixed(declared, "LogLevel"),
		},
		{
			page: docsSlicesPage, sentence: "can disagree in three ways", spelled: 3,
			members: membersOfTheSurfacePrefixed(declared, "Finding"),
		},
		{
			page: docsGraphPage, sentence: "every field is one a caller legitimately wants alone",
			members: declared.fieldsOf("graph/projection", "Summary"),
		},
		{
			page: docsPatternsPage, sentence: "four of them are exported", spelled: 4,
			members: exportedProjections(declared),
		},
		{
			page: docsGrammarPage, sentence: "The two families without a second spelling", spelled: 5,
			members: membersOfTheSurfacePrefixed(declared, "Project"),
		},
	} {
		if !strings.Contains(readDocument(t, enumerated.page), enumerated.sentence) {
			t.Errorf("%s no longer says %q, so nothing here holds it to the set it enumerates",
				enumerated.page, enumerated.sentence)
		}
		if len(enumerated.members) == 0 {
			t.Errorf("this library declares nothing that %q is about, so that sentence of %s is pinned by "+
				"nothing at all", enumerated.sentence, enumerated.page)
			continue
		}
		if enumerated.spelled != 0 && len(enumerated.members) != enumerated.spelled {
			t.Errorf("%s says %q and this library has %d of them: %v",
				enumerated.page, enumerated.sentence, len(enumerated.members), enumerated.members)
		}

		named := namesTheDocumentNames(t, enumerated.page)
		for _, member := range enumerated.members {
			if !named[member] {
				t.Errorf("%s says %q and names no %s: a set a page states as a list of its members is silently "+
					"incomplete the day one is added", enumerated.page, enumerated.sentence, member)
			}
		}
	}

	// The two dashes of the entry-point table, restated in prose on that page and again on slices.md and
	// metrics.md. `Slices` is the alias grammar.md says the slices family has none of, and `Graph` the one
	// it says the dependency-graph family spells `DependencyGraph` instead — and exporting either leaves
	// every count above right, because an alias is one more package-level function and none of these
	// sentences is about how many there are.
	for _, absent := range []string{"Slices", "Graph"} {
		if declared.surface[absent] {
			t.Errorf("%s exports %s, and %s states archunit.Project%s and no such second spelling of it: a "+
				"family that grew an alias has an entry-point table nothing else here can fail on",
				surfaceFile, absent, docsGrammarPage, absent)
		}
	}
}

// TestTheLandingPageInstallsThisModuleTheWayGoModDescribesIt pins the three facts of the install section
// that live in go.mod, for the reason the README's are pinned: the path a user types, the Go version they
// need, and every dependency they take on with it. The site states them because it is where a reader who
// arrived from a search engine starts, and a second copy of an install instruction is a second one to go
// wrong.
func TestTheLandingPageInstallsThisModuleTheWayGoModDescribesIt(t *testing.T) {
	landing := readDocument(t, docsLandingPage)

	for _, required := range []string{
		"go get " + modulePathOfThisRepository(t),
		"Go " + valueOfTheModuleDirective(t, "go") + " or newer",
	} {
		if !strings.Contains(landing, required) {
			t.Errorf("%s does not say %q, which is what %s says", docsLandingPage, required, moduleFile)
		}
	}
	for _, dependency := range directRequirementsOfThisRepository(t) {
		if !strings.Contains(landing, dependency) {
			t.Errorf("%s takes a direct dependency on %s and %s does not mention it: the install section says "+
				"which dependencies come with this library", moduleFile, dependency, docsLandingPage)
		}
	}
}

// TestTheDocsSiteIsConfiguredForTheRepositoryItDocuments reads the site's settings off go.mod rather than
// trusting them. A project site is served under the repository's own name, so a wrong baseurl is a site
// whose every asset 404s; a wrong repository or reference URL is the two links every page carries pointing
// at somebody else's project.
func TestTheDocsSiteIsConfiguredForTheRepositoryItDocuments(t *testing.T) {
	module := modulePathOfThisRepository(t)
	segments := strings.Split(module, "/")
	repository := segments[len(segments)-1]

	for setting, expected := range map[string]string{
		"title":             repository,
		"baseurl":           "/" + repository,
		"repository_url":    "https://" + module,
		"api_reference_url": "https://pkg.go.dev/" + module,
	} {
		if stated := valueOfTheSiteSetting(t, setting); stated != expected {
			t.Errorf("%s states %s: %s, and %s says this module is %s",
				docsConfigFile, setting, stated, moduleFile, expected)
		}
	}

	// The pages link to each other by filename, which only works on the built site because this plugin
	// rewrites those links. Losing it breaks every internal link at once, and only once published.
	config := readDocument(t, docsConfigFile)
	for _, required := range []string{"jekyll-relative-links", "relative_links:", "enabled: true"} {
		if !strings.Contains(config, required) {
			t.Errorf("%s does not state %q: the pages of this site link to each other by filename, and that is "+
				"what rewrites those links for the built site", docsConfigFile, required)
		}
	}
}

// TestTheLayoutOfTheDocsSiteNamesNoPageOfIt pins the one design decision in the layout: the navigation is
// generated from the pages' front matter, so a page added to docs/ appears in it and a page removed leaves
// it. A hardcoded list would be the one part of this site that could silently disagree with what is in the
// folder — and the test above, which holds every page to having a nav_order, would be checking a key
// nothing read.
//
// It also pins the two Liquid tags that put a page on its own page, because every other test in this file
// reads the Markdown sources: a layout that stopped pouring the content in would publish ten banners and a
// navigation, and nothing here would notice.
func TestTheLayoutOfTheDocsSiteNamesNoPageOfIt(t *testing.T) {
	layout := readDocument(t, docsLayoutFile)

	if !strings.Contains(layout, "{{ content }}") {
		t.Errorf("%s pours no page into itself: without that one tag every page of this site publishes as a "+
			"banner and a navigation with none of its own prose under them", docsLayoutFile)
	}
	if !strings.Contains(layout, "{{ page.title }}") {
		t.Errorf("%s does not headline the browser tab with the page's own title, so the front matter this file "+
			"holds every page to is a key the layout reads for the navigation alone", docsLayoutFile)
	}
	if !strings.Contains(layout, "nav_order") {
		t.Errorf("%s builds no navigation from nav_order, and every page of this site states one",
			docsLayoutFile)
	}
	for _, documented := range pagesOfTheDocsSite(t) {
		if strings.Contains(layout, documented.name) {
			t.Errorf("%s names the page %s: the navigation is generated from front matter so that adding a "+
				"page is adding a file", docsLayoutFile, documented.name)
		}
	}

	// The site is served under a baseurl, so every asset and every internal link goes through Jekyll's
	// relative_url filter. An absolute path resolves above the site and 404s.
	if !strings.Contains(layout, "'/assets/docs.css' | relative_url") {
		t.Errorf("%s does not link the stylesheet through relative_url: a path written absolutely resolves "+
			"above a project site", docsLayoutFile)
	}
	if strings.Contains(layout, `href="/`) {
		t.Errorf("%s writes an absolute href: under a baseurl that points above the site", docsLayoutFile)
	}
	if _, err := os.Stat(docsStylesheet); err != nil {
		t.Errorf("%s links a stylesheet this repository does not hold at %s: %v",
			docsLayoutFile, docsStylesheet, err)
	}
}

// TestThePagesWorkflowPublishesTheDocsFolderAndChecksNothing is the workflow held to the one thing it does.
//
// The split matters. This file is the check, and it runs under `go test ./...` on every push to every
// branch, because a page that names a verb this library does not have is wrong before it is published
// rather than after. The publish workflow watches main alone — so if it were also the check, the check
// would run after the merge, which is the definition of not being a gate.
//
// It also pins the split between building the site and publishing it. Pages is a repository setting, and
// this repository does not have it turned on; the workflow reads that rather than failing on it, so the
// build runs on every push and the deployment waits for the setting. What the test holds is that the
// waiting is spelled as a condition on the deploy alone — not as a step that cannot fail, and not as a
// build that never runs.
func TestThePagesWorkflowPublishesTheDocsFolderAndChecksNothing(t *testing.T) {
	workflow := collapsed(readDocument(t, pagesWorkflow))

	for _, required := range []string{
		// The site source is docs/ and not the repository root, so README.md is not a page of it.
		"source: ./docs",
		// The four actions of a Pages deployment: what the baseurl is, the build, the artifact, the deploy.
		"actions/configure-pages@v",
		"actions/jekyll-build-pages@v",
		"actions/upload-pages-artifact@v",
		"actions/deploy-pages@v",
		// A publish is not a check, and a branch's half-written page is not what the site should show.
		"branches: [main]",
		"if: github.ref == 'refs/heads/main'",
		// Whether Pages is switched on is a setting no workflow can set, so the build asks and the publish
		// waits on the answer. Both halves are pinned: the question the build passes on, and the condition
		// the deployment reads it as.
		"pages_enabled: ${{ steps.site.outputs.enabled }}",
		"needs.build.outputs.pages_enabled == 'true'",
		// A half-published site is worse than a stale one, so a deployment is never canceled mid-upload.
		"group: pages",
		"cancel-in-progress: false",
		// The token that can publish belongs to the deploy job; the build only reads the repository.
		"contents: read",
		"pages: write",
		"id-token: write",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("%s does not say %q", pagesWorkflow, required)
		}
	}

	for _, switchedOff := range []string{"continue-on-error", "if: false"} {
		if strings.Contains(workflow, switchedOff) {
			t.Errorf("%s uses %q: a step that cannot fail publishes a site nobody built", pagesWorkflow,
				switchedOff)
		}
	}

	// The setting gates the publish and nothing else. A build behind it would mean the one part of this
	// workflow that can be checked without an administrator — that GitHub's Jekyll renders docs/ at all —
	// only ran once it no longer mattered.
	if strings.Contains(workflow, "enabled == 'true' uses: actions/jekyll-build-pages@v") {
		t.Errorf("%s builds the site only when Pages is enabled: the build is the half of this workflow that "+
			"needs no setting, and skipping it leaves a broken site undiscovered until the day it is published",
			pagesWorkflow)
	}
	for _, floating := range []string{"@main", "@master"} {
		if strings.Contains(workflow, floating) {
			t.Errorf("%s pins an action to %s: somebody else's push would change what this repository "+
				"publishes", pagesWorkflow, floating)
		}
	}
	for _, command := range gateOfThisRepository() {
		if strings.Contains(workflow, command) {
			t.Errorf("%s runs %q: the gate is %s, which watches every branch, and a check that only runs "+
				"after a merge to main is not a gate", pagesWorkflow, command, ciWorkflow)
		}
	}
}

// TestTheReadmePointsAtTheDocsSite is the last link in the chain. The README is what a reader skims before
// deciding, and the site is what they read while writing rules — so a site nothing links to is nine pages
// found by whoever already knew they were there.
//
// The URL is assembled from the site's own configuration rather than spelled here, because the two halves
// of it are the same two the built site is served under.
func TestTheReadmePointsAtTheDocsSite(t *testing.T) {
	site := valueOfTheSiteSetting(t, "url") + valueOfTheSiteSetting(t, "baseurl") + "/"

	if !strings.Contains(readDocument(t, readmeFile), site) {
		t.Errorf("%s does not link to %s, which is where %s is published", readmeFile, site, docsFolder)
	}
}

// sitePage is one page of the documentation site as the tests above need it: where it is, what it calls
// itself in its front matter, and what it calls itself in its own first heading.
type sitePage struct {
	// path is the page as a path from the repository root — docs/files.md.
	path string
	// name is the page as a link between two pages spells it — files.md.
	name string
	// frontMatter is the page's YAML header, which is how it gets a layout, a title and a place in the
	// navigation.
	frontMatter map[string]string
	// heading is the page's first heading, which is the title a reader actually sees.
	heading string
}

// pagesOfTheDocsSite is every page of the site: each Markdown file directly in docs/, in the order the
// folder lists them. There is no list of pages anywhere in this repository — not in the layout, not in the
// configuration and not here — because a page is a file, and a list is the thing that disagrees with the
// folder.
func pagesOfTheDocsSite(t *testing.T) []sitePage {
	t.Helper()

	entries, err := os.ReadDir(docsFolder)
	if err != nil {
		t.Fatalf("reading %s failed: %v", docsFolder, err)
	}

	var pages []sitePage
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := docsFolder + "/" + entry.Name()
		pages = append(pages, sitePage{
			path:        path,
			name:        entry.Name(),
			frontMatter: frontMatterOf(t, path),
			heading:     firstHeadingOf(t, path),
		})
	}
	if len(pages) < 2 {
		t.Fatalf("%s holds %d page(s): this site is a guide, and a guide is more than one document",
			docsFolder, len(pages))
	}
	return pages
}

// navOrderOf is the place a page gives itself in the navigation, and whether it gave itself one at all.
func navOrderOf(documented sitePage) (int, bool) {
	order, err := strconv.Atoi(documented.frontMatter["nav_order"])
	return order, err == nil
}

// frontMatterOf is a page's YAML header as the keys the layout reads it by: everything between the opening
// `---` and the next one, one `key: value` to a line.
//
// It is parsed rather than searched for, because what the tests need is the value — the title the
// navigation shows, the order it shows it in — and because a page with no header at all has to come back
// as nothing rather than as a plausible-looking guess.
func frontMatterOf(t *testing.T, document string) map[string]string {
	t.Helper()

	stated := map[string]string{}
	opened := false
	for line := range strings.Lines(readDocument(t, document)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if opened {
				return stated
			}
			opened = true
			continue
		}
		if !opened {
			break // The header is the first thing in the file or it is not a header at all.
		}
		if key, value, isPair := strings.Cut(trimmed, ":"); isPair {
			stated[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return map[string]string{}
}

// firstHeadingOf is the title a reader sees at the top of a page, as opposed to the one in its front
// matter. The two agreeing is what TestEveryPageOfTheDocsSiteSaysWhereItBelongs is about.
func firstHeadingOf(t *testing.T, document string) string {
	t.Helper()

	if headings := headingsOf(t, document); len(headings) > 0 {
		return headings[0]
	}
	return ""
}

// headingsOf are a page's headings, in order, with their `#`s taken off. They are read out of the prose so
// that a comment in an example is not a heading.
func headingsOf(t *testing.T, document string) []string {
	t.Helper()

	var headings []string
	for line := range strings.Lines(proseOf(t, document)) {
		if text, isHeading := strings.CutPrefix(strings.TrimSpace(line), "#"); isHeading {
			headings = append(headings, strings.TrimSpace(strings.TrimLeft(text, "#")))
		}
	}
	return headings
}

// anchorOf is the fragment a heading can be linked at: its text lowercased, with every run of anything
// else turned into one hyphen. `## Mood` is reached as `grammar.md#mood`, which is what a link between two
// pages of this site points at.
func anchorOf(heading string) string {
	return strings.Trim(notInAnAnchor.ReplaceAllString(strings.ToLower(heading), "-"), "-")
}

// linkTargetsOf is every target a page links to, in the order they appear: what stands in the parentheses
// of each Markdown link. The whole document is read rather than only its prose, because a link inside an
// example would be just as broken.
func linkTargetsOf(t *testing.T, document string) []string {
	t.Helper()

	var targets []string
	for _, link := range markdownLink.FindAllStringSubmatch(readDocument(t, document), -1) {
		targets = append(targets, strings.TrimSpace(link[1]))
	}
	return targets
}

// valueOfTheSiteSetting is a top-level setting of the site's configuration. Only a top-level key matches,
// because the value of an indented one belongs to the key above it and is not the site's own answer to
// anything.
func valueOfTheSiteSetting(t *testing.T, key string) string {
	t.Helper()

	for line := range strings.Lines(readDocument(t, docsConfigFile)) {
		if value, stated := strings.CutPrefix(line, key+":"); stated {
			return strings.TrimSpace(value)
		}
	}
	t.Fatalf("%s states no %s", docsConfigFile, key)
	return ""
}

// verbsOf are the exported methods of one stage of one fluent package that hand back one particular thing,
// which is how every count these pages state is counted: a modifier hands back its own builder, a metric
// verb hands back the stage that can be resolved, a threshold hands back a condition.
//
// An exclusion is never one of them. `Except` qualifies the verb in front of it, so it is part of a clause
// rather than a verb a reader picks from a list.
func verbsOf(declared declarations, pkg, receiver, returns string) []string {
	var found []string
	for _, method := range declared.methodsOf(pkg) {
		if method.receiver != receiver || strings.HasPrefix(method.name, "Except") {
			continue
		}
		if method.results() == returns {
			found = append(found, method.name)
		}
	}
	return found
}

// exclusionsOf are the exclusion verbs one family offers, as the distinct names of them: `Except` and
// whichever targeted forms the family's own selectors name.
//
// Distinct, because a family declares each of them on every selector that has one — files/fluentapi spells
// the same four on its scope stage and again on its dependency clause — while a page saying a family offers
// four of them is counting the vocabulary a reader has to learn, which is the name and not the receiver.
func exclusionsOf(declared declarations, pkg string) []string {
	var found []string
	for _, method := range declared.methodsOf(pkg) {
		if strings.HasPrefix(method.name, "Except") && !slices.Contains(found, method.name) {
			found = append(found, method.name)
		}
	}
	slices.Sort(found) // A failure naming them in declaration order would name them differently per receiver.
	return found
}

// exportedProjections are the `MapFunction` factories the public surface hands a caller who wants to hold
// the slicing a fluent verb builds: the `SliceBy` family and `Identity`, which is the one of them not named
// after a slicing because it is the absence of one.
//
// `Identity` is looked up by name rather than by a prefix because it is a whole name, and it is looked up
// rather than assumed so that the count goes red if it stops being exported as well as when a fifth
// projection joins it.
func exportedProjections(declared declarations) []string {
	found := membersOfTheSurfacePrefixed(declared, "SliceBy")
	if declared.surface["Identity"] {
		found = append(found, "Identity")
	}
	return found
}

// membersOfTheSurfacePrefixed are the public surface's members of one closed set, found by the prefix every
// member of it shares: `Kind` is the violation kinds, `Color` the colors a report is painted in. It is how a
// page that states such a set as a list is held to the list rather than to its own count.
//
// The name equal to the prefix is not one of them. That is the type the members are of — `Color`, `LogLevel` —
// which a page names as the set itself and not as a member of it.
func membersOfTheSurfacePrefixed(declared declarations, prefix string) []string {
	var found []string
	for name := range declared.surface {
		if name != prefix && strings.HasPrefix(name, prefix) {
			found = append(found, name)
		}
	}
	slices.Sort(found) // The surface is a map, and a failure naming them in a different order every run is noise.
	return found
}

// terminalsOf are the methods of one stage that hand something back and can fail — which is what a
// terminal is, in every family: the one stage that reads the project.
func terminalsOf(declared declarations, pkg, receiver string) []string {
	var found []string
	for _, method := range declared.methodsOf(pkg) {
		if method.receiver == receiver && strings.HasSuffix(method.results(), "error") {
			found = append(found, method.name)
		}
	}
	return found
}

// methodsEveryFamilyShares are the three methods that let a chain describe itself rather than judge
// anything. They are not on every stage of every family — `String` is on all but LayerPolicyBuilder, `Mood`
// on the mood stage the files and slices families have, `Selectors` on the files and metrics scope stages —
// which is why the exemption is by name: wherever a family declares one, docs/grammar.md is where it is
// documented, for all five at once. That is why a family page is not asked to name them, and why the test
// that exempts them also holds that page to naming them.
func methodsEveryFamilyShares() []string {
	return []string{"String", "Mood", "Selectors"}
}

// markdownLink is a Markdown link, and the group is what it points at. Only this spelling is looked for:
// a raw HTML link is a finding of its own, because the built site does not rewrite one.
var markdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// notInAnAnchor is every run of characters a heading's fragment turns into a hyphen.
var notInAnAnchor = regexp.MustCompile(`[^a-z0-9]+`)
