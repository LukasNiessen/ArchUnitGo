package calculation

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
)

// LCOM96a is the lack of cohesion of a class as Henderson-Sellers spelled it in 1996: how far the average
// field falls short of being reached by every method.
//
//	LCOM96a = ((1/a) * Σ μ(A) - m) / (1 - m)
//
// 0 is perfect cohesion, where every method reaches every field. 1 is a class whose fields are each reached by
// exactly one method — the shape a class that is really two classes has. It is not bounded above: a class whose
// methods reach no field at all scores m/(m-1), which is above 1 and is meant to be, because such a class has
// no field holding it together at all.
//
// A class with no fields, or with fewer than two methods, is 0: the question cannot be asked of it, which is not
// the same as perfect cohesion.
func LCOM96a(class extraction.ClassInfo) float64 {
	if !measurable(class) {
		return 0
	}
	methods := float64(len(class.Methods))
	average := accessCount(class) / float64(len(class.Fields))
	return (average - methods) / (1 - methods)
}

// LCOM96b is the other lack of cohesion Henderson-Sellers spelled in 1996: the average fraction of the class's
// methods that do not reach a field.
//
//	LCOM96b = (1/a) * Σ ((m - μ(A)) / m)
//
// It is LCOM96a normalised by the number of methods rather than by one fewer, which is what bounds it: 0 is
// perfect cohesion and 1 is a class whose methods reach no field at all. Each field being reached by exactly
// one method scores 1 - 1/m, so the two 1996 measures disagree about how bad that is and agree about the ends.
//
// A class with no fields, or with fewer than two methods, is 0: the question cannot be asked of it, which is not
// the same as perfect cohesion.
func LCOM96b(class extraction.ClassInfo) float64 {
	if !measurable(class) {
		return 0
	}
	pairings := float64(len(class.Fields) * len(class.Methods))
	return 1 - accessCount(class)/pairings
}

// LCOM1 is the lack of cohesion Chidamber & Kemerer counted in pairs: the pairs of methods that reach no field
// in common, less the pairs that reach one, and never below 0.
//
//	LCOM1 = max(0, |{disjoint pairs}| - |{sharing pairs}|)
//
// 0 is a class whose methods share at least as often as they do not, and it is the answer for most classes that
// hold together. It is a count rather than a ratio, so it grows with the square of the number of methods: a
// threshold written against it is about one class's size as much as about its cohesion, and LCOMStar is the same
// question asked as a ratio.
//
// A class with no fields, or with fewer than two methods, is 0: the question cannot be asked of it, which is not
// the same as perfect cohesion.
func LCOM1(class extraction.ClassInfo) int {
	if !measurable(class) {
		return 0
	}
	sharing, disjoint := methodPairs(class)
	return max(0, disjoint-sharing)
}

// LCOM2 is the lack of cohesion Chidamber & Kemerer normalised in 1994: the fraction of the class's
// method-field pairings that are not accesses.
//
//	LCOM2 = 1 - Σ μ(A) / (m * a)
//
// It is LCOM96b's exact twin. Expand LCOM96b's average of (m - μ(A))/m over the a fields and the two are one
// expression, and this library keeps both names because they come from different papers and a user arriving
// from either looks for the one they know. They are one function here so that they cannot drift apart.
//
// A class with no fields, or with fewer than two methods, is 0: the question cannot be asked of it, which is not
// the same as perfect cohesion.
func LCOM2(class extraction.ClassInfo) float64 {
	return LCOM96b(class)
}

// LCOM3 is the lack of cohesion Li & Henry normalised in 1993: how far the average field falls short of being
// reached by every method, over one fewer than the methods.
//
//	LCOM3 = (m - (1/a) * Σ μ(A)) / (m - 1)
//
// It is LCOM96a's exact twin — negate LCOM96a's numerator and its denominator — and it is kept under its own
// name for the reason LCOM2 is: the name is what a user coming from the literature searches for. LCOM5 is the
// third of the ratios of averages and is not a twin of either: it averages over the fields where these two
// average over the methods.
//
// A class with no fields, or with fewer than two methods, is 0: the question cannot be asked of it, which is not
// the same as perfect cohesion.
func LCOM3(class extraction.ClassInfo) float64 {
	return LCOM96a(class)
}

// LCOM4 is the lack of cohesion Hitz & Montazeri counted in 1995: how many pieces the class would fall into if
// it were split wherever its methods share no field.
//
// The methods are the nodes of a graph, two of them are joined when they reach a field in common, and the
// number is how many connected components that graph has. 1 is a class that holds together; 2 is a class that
// is two classes sharing a name, and each component names the methods and fields one of them would take with
// it.
//
// The 1995 measure also joins a method to a method it calls. This library does not, and the graph here is the
// shared-field one alone: which methods call which is a second relation, extraction.ClassInfo keeps only the
// field one, and the sibling ports count the components of the same graph — so a class scores the same in every
// one of them. It makes this number an upper bound on the paper's: a class the paper would call one piece can
// be two here, never the other way round.
//
// A class with no fields, or with fewer than two methods, is 1, and 0 for a class with no methods at all: the
// question cannot be asked of it, which is not the same as a class that holds together.
func LCOM4(class extraction.ClassInfo) int {
	if len(class.Methods) == 0 {
		return 0
	}
	if !measurable(class) {
		return 1
	}

	component := make([]int, len(class.Methods))
	for index := range component {
		component[index] = index
	}
	reached := make(map[string]int, len(class.Fields))
	for index, method := range class.Methods {
		for _, field := range method.AccessedFields {
			if first, seen := reached[field]; seen {
				connect(component, index, first)
				continue
			}
			reached[field] = index
		}
	}

	components := 0
	for index := range component {
		if rootOf(component, index) == index {
			components++
		}
	}
	return components
}

// LCOM5 is the lack of cohesion Henderson-Sellers normalised over the fields in 1996: how far the average
// method falls short of reaching every field.
//
//	LCOM5 = (a - (1/m) * Σ μ(A)) / (a - 1)
//
// It is the dual of LCOM96a — the average is over the methods of a field there and over the fields of a method
// here — so a class with many methods and few fields scores differently on the two. 0 is perfect cohesion and 1
// is a class whose methods each reach one field of their own; a class most of whose fields nobody reaches scores
// above 1, for the reason LCOM96a can.
//
// A class with a single field is 0: there is no spread across fields left to normalise, and the shape of the
// formula would divide by zero rather than say so. LCOM96a and the pair measures are the ones to write a rule
// with about a class that has one field.
//
// A class with no fields, or with fewer than two methods, is 0 for the same reason: the question cannot be asked
// of it, which is not the same as perfect cohesion.
func LCOM5(class extraction.ClassInfo) float64 {
	fields := len(class.Fields)
	if !measurable(class) || fields < 2 {
		return 0
	}
	average := accessCount(class) / float64(len(class.Methods))
	return (float64(fields) - average) / float64(fields-1)
}

// LCOMStar is LCOM*, the lack of cohesion Fernández & Peña stated as a ratio in 2006: the share of the method
// pairs that reach no field in common.
//
//	LCOM* = |{disjoint pairs}| / |{pairs}|
//
// It is LCOM1's question with the size of the class divided out, which is what makes a threshold written
// against it mean the same thing for a class of three methods and one of thirty. 0 is a class whose every pair
// of methods shares a field, and 1 is a class no two of whose methods share one.
//
// A class with no fields, or with fewer than two methods, is 0: the question cannot be asked of it, which is not
// the same as perfect cohesion.
func LCOMStar(class extraction.ClassInfo) float64 {
	if !measurable(class) {
		return 0
	}
	sharing, disjoint := methodPairs(class)
	return float64(disjoint) / float64(sharing+disjoint)
}

// measurable reports whether a lack of cohesion can be read off this class at all: it takes a field for methods to
// share and two methods to share it or fail to.
//
// It is one guard for the whole family, and every metric above answers "no evidence" when it does not hold —
// 0 for the seven whose 0 is perfect cohesion, and 1 for LCOM4, whose scale starts at one component. That is
// deliberately more than the arithmetic needs. A class with one method has no pair of methods to be incohesive
// between, and a class with no fields — every interface, and every type that is not a struct — has nothing its
// methods could share, so counting each of its methods as its own component would report the whole population
// of Go interfaces as maximally incohesive, in a way no user could fix.
//
// A class with no methods at all is 0 on every scale, LCOM4 included: there is nothing there to fall apart.
func measurable(class extraction.ClassInfo) bool {
	return len(class.Fields) > 0 && len(class.Methods) > 1
}

// accessCount is Σ μ(A), the total number of accesses the class's methods make to its fields, counting a field
// once per method that reaches it. It is a float64 because every formula above divides it.
func accessCount(class extraction.ClassInfo) float64 {
	total := 0
	for _, field := range class.Fields {
		total += len(field.AccessedBy)
	}
	return float64(total)
}

// methodPairs counts the unordered pairs of this class's methods that reach a field in common, and the pairs
// that reach none. Together they are every pair, which is what lets LCOM1 subtract the one from the other and
// LCOMStar divide.
func methodPairs(class extraction.ClassInfo) (sharing, disjoint int) {
	for left := range class.Methods {
		for right := left + 1; right < len(class.Methods); right++ {
			if sharesField(class.Methods[left], class.Methods[right]) {
				sharing++
				continue
			}
			disjoint++
		}
	}
	return sharing, disjoint
}

// sharesField reports whether two methods reach a field in common. The lists are the fields of one class, so they
// are short, and a scan of one against the other is what the pair measures are defined over.
func sharesField(left, right extraction.MethodInfo) bool {
	for _, field := range left.AccessedFields {
		if slices.Contains(right.AccessedFields, field) {
			return true
		}
	}
	return false
}

// connect joins the components of two methods in the disjoint-set forest LCOM4 counts the components of. The
// forest is a slice of method indexes, each holding the index of its parent, and a root holds its own.
func connect(component []int, left, right int) {
	leftRoot, rightRoot := rootOf(component, left), rootOf(component, right)
	if leftRoot != rightRoot {
		component[rightRoot] = leftRoot
	}
}

// rootOf is the method that represents another method's component, flattening the path it walked on the way so
// that the next walk is shorter.
func rootOf(component []int, index int) int {
	for component[index] != index {
		component[index] = component[component[index]]
		index = component[index]
	}
	return index
}
