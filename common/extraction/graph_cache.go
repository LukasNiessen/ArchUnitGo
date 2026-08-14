package extraction

import (
	"fmt"
	"slices"
	"sync"
)

// CachedGraph is the door a check goes through to get a project's dependency graph: it returns the
// graph of the project at root, and runs the extractor only when no earlier call has already extracted
// exactly these inputs.
//
// root is a host path, normally the one LocateProject returned, and options are the ones ExtractGraph
// takes. The result is that function's result, with the same invariants — this is a memo in front of
// it and nothing else. Extraction runs the Go toolchain once over the whole project and then parses
// every file the walk enumerated, while a real test suite runs dozens of rules over one project, so
// for the second rule onwards this is a map lookup instead.
//
// Three things a caller has to know:
//
//   - The cache is keyed on every input that can change the graph — the project root, the folder
//     exclusions and the analysis toggles. graphCacheKey builds that key, in one place, which is what
//     makes asking for the same project under different options an extraction rather than the wrong
//     graph handed back. Two spellings of one project are one entry: the root is resolved before it is
//     keyed on, the way ExtractGraph resolves it, so a relative root and a linked one land on the
//     canonical directory.
//   - A failure is not cached. An extraction that failed did so because of the environment — an
//     unreadable root, a toolchain that could not be run — and the next rule in the suite deserves the
//     same chance rather than a memoised error; the cost of retrying is the cost of the failure.
//   - The graph handed back is the caller's own copy. A Graph is a slice, so a caller writing through
//     it would otherwise reach into every later reader's graph; the cache keeps its copy and hands out
//     others.
//
// Entries live as long as the process, which for a test binary asking about one project means one
// entry. ClearGraphCache is the escape hatch, and CheckOptions.ClearCache is the same hatch on a rule.
func CachedGraph(root string, options *SourceOptions) (Graph, error) {
	return sharedGraphCache.graph(root, options)
}

// ClearGraphCache throws away every graph the cache holds, so that the next check extracts its project
// from source again. It is the global escape hatch from the memo CachedGraph is, and CheckOptions has
// the same hatch per rule.
//
// Reach for it when the source changed underneath the library and a graph extracted earlier in the
// process no longer describes it: a test that writes a fixture project, code generated between two
// checks, a long-running host that wants the memory back. Nothing else invalidates an entry — the
// library does not watch the filesystem, because a check that stats every file of a project to decide
// whether to trust the cache has already paid most of the cost of extraction.
//
// It clears the whole cache rather than one project's entry, because the reason to clear is that the
// source moved, and the same project is cached once per set of options it was asked about. It is safe
// to call from any goroutine, and calling it on an empty cache does nothing.
func ClearGraphCache() {
	sharedGraphCache.clear()
}

// sharedGraphCache is the process-wide memo the two functions above are the surface of. A cache exists
// to be shared by every rule in a suite, and rules are built and checked independently of each other,
// so there is no value for it to hang from — which is exactly why ClearGraphCache is public.
//
//nolint:gochecknoglobals // the process-wide graph memo this file exists to be; ClearGraphCache resets it.
var sharedGraphCache graphCache

// graphCache memoises extracted graphs by the inputs that produced them. The zero value is an empty
// cache, so the map is built on the first store and dropped again by clear.
//
// It is a type rather than a bare map so that the mutex, the key and the copying live with it. Every
// method takes a pointer receiver: the mutex makes the value uncopyable.
type graphCache struct {
	// mutex guards graphs. A check is synchronous, but nothing stops two tests running in parallel from
	// asking about the same project, and a map read racing a map write is a crash rather than a stale
	// answer.
	mutex sync.Mutex
	// graphs is the memo: a key from graphCacheKey to the graph extracted under it.
	graphs map[string]Graph
}

// graph returns the cached graph for these inputs, extracting the project when there is none. It is
// deliberately not holding the lock across the extraction: a second goroutine asking about the same
// project while the first is still in the toolchain extracts it too and stores the same answer, which
// costs one redundant extraction in a case that does not arise in a test suite, while locking for the
// whole extraction would make one project's rules wait on another's.
func (c *graphCache) graph(root string, options *SourceOptions) (Graph, error) {
	directory, err := resolveProjectRoot(root)
	if err != nil {
		return nil, err
	}

	key := graphCacheKey(directory, options)
	if cached, found := c.lookup(key); found {
		return cached, nil
	}

	graph, err := ExtractGraph(directory, options)
	if err != nil {
		return nil, err
	}
	c.store(key, graph)
	return graph, nil
}

// lookup returns a copy of the graph stored under key, if there is one.
func (c *graphCache) lookup(key string) (Graph, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cached, found := c.graphs[key]
	if !found {
		return nil, false
	}
	return slices.Clone(cached), true
}

// store keeps a copy of graph under key. The copy is what makes the graph the caller was handed theirs
// to keep.
func (c *graphCache) store(key string, graph Graph) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.graphs == nil {
		c.graphs = make(map[string]Graph, 1)
	}
	c.graphs[key] = slices.Clone(graph)
}

// clear empties the cache.
func (c *graphCache) clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.graphs = nil
}

// graphCacheKey is the one place a cache key is built, and the reason it is a named function of its own
// is that forgetting an input here is the one way this cache can be wrong: it would hand a rule the
// graph of a *different* analysis of the same project, and every rule downstream would be judging
// source that was never read that way.
//
// So it names every input ExtractGraph's result depends on: the project root, and every field of
// SourceOptions.
//
//   - the resolved root, which is what a locator resolved to — two locators pointing into one project
//     are one key, and that is the point;
//   - ExcludedFolders, which decides what the walk enumerates;
//   - IncludeTestFiles and BuildTags, which decide what the toolchain puts in the build;
//   - IgnoredImportKinds, which decides which imports become edges.
//
// Nothing else reaches the extractor, and graph_cache_test.go fails if a field is added to
// SourceOptions without arriving here.
//
// The options are resolved first, so that the three ways of spelling the defaults — a nil bag, a zero
// bag, and one whose exclusions are nil — are one key, while a caller who excluded nothing on purpose
// keeps their own. Both lists are sorted, because neither one's order changes what is extracted: an
// exclusion list is a set, and `-tags=a,b` is the same build as `-tags=b,a`. Every string is quoted, so
// that a folder named `a b` cannot forge a boundary between two entries.
func graphCacheKey(root string, options *SourceOptions) string {
	resolved := options.WithDefaults()
	folders := slices.Sorted(slices.Values(resolved.ExcludedFolders))
	tags := slices.Sorted(slices.Values(resolved.BuildTags))

	return fmt.Sprintf(
		"root=%q\nfolders=%q\ntests=%t\ntags=%q\nignored=%d",
		root, folders, resolved.IncludeTestFiles, tags, uint8(resolved.IgnoredImportKinds),
	)
}
