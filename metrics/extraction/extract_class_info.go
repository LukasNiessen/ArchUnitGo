package extraction

import (
	"go/ast"
	"go/token"
	"slices"
)

// FieldInfo is one field of a class, and which of the class's methods reach it.
//
// It is one half of the relation every cohesion metric is a formula over — μ(A), the number of methods that
// access a field, is len(AccessedBy) — and MethodInfo is the other. The two are one fact written down twice
// because the family asks it both ways: LCOM96a and its kin count the methods each field is reached by, while
// the pair formulas ask which fields two methods have in common. They are built together, from one walk of
// one syntax tree, so they cannot disagree.
type FieldInfo struct {
	// Name is the field as a method reaches it — `name`, of `h.name`. For an embedded field it is the name of
	// the embedded type without its package qualifier, its pointer star or its type arguments, because that
	// is what a method selects on the receiver: `Reader`, of an embedded `*io.Reader`.
	Name string
	// AccessedBy names the methods of this class that select this field on their receiver, in the order those
	// methods were declared and each of them once however often it reaches the field. It is empty for a field
	// no method of the class reaches.
	AccessedBy []string
}

// MethodInfo is one method of a class, and which of the class's fields it reaches. It is the other half of the
// relation FieldInfo describes above.
type MethodInfo struct {
	// Name is the declared name of the method — `Handle`, of `func (h *Handler) Handle()`.
	Name string
	// AccessedFields names the fields of this class the method selects on its receiver, in the order it first
	// selects each of them and each of them once. It is empty for a method that reaches none.
	//
	// Only the class's own declared fields are in here. A name the method selects on its receiver that is not
	// one of them — a method call, a field promoted from an embedded type — is not an access this package can
	// attribute to a field, because it would have to resolve a type to know what it is.
	AccessedFields []string
}

// ClassInfo is one type a file declares, the two counts a rule can be written about it — how many fields it
// has and how many methods — and which of its fields each of its methods reaches.
//
// Go has no classes. The vocabulary is the family's — matching.TargetClassname says the same thing about
// the scope verb `for classes matching` — and in this library a class is a declared type: a struct, an
// interface, or a name given to another type. That is the population `for classes matching` selects and
// the population `count, classes` counts, so the word means one thing everywhere.
//
// Two of the family's class-level metrics are deliberately absent rather than faked, because Go has
// nothing for them to be about: there is no inheritance, so depth of inheritance and number of children
// have no answer here, and a metric returning 0 for every type in every project would be a rule that
// passes forever while looking like it holds.
type ClassInfo struct {
	// Name is the declared name on its own — `Handler`. It is what a reader of a report recognizes, and
	// what a method's receiver names.
	Name string
	// Identifier is the name qualified by the folder the type was declared in — `internal/api.Handler`,
	// and `Handler` for a type declared at the project root.
	//
	// It is the string a class metric reports as its subject and the one `for classes matching` is
	// matched against: matching.TargetClassname strips exactly this qualification off before comparing,
	// so a pattern is written about the bare name while the report still says which package the type was
	// in. Two packages declaring `Handler` are two subjects, which is the whole reason the qualification
	// is carried at all.
	Identifier string
	// Path is the identifier of the file the type was declared in — `internal/api/handler.go`. A class
	// metric reports the class, and this is how a caller gets from it to the file to open.
	Path string
	// Interface says whether the declared type is an interface. It is what `count, interfaces` counts,
	// and the reason a metric about interfaces needs no second population: an interface is a class, so
	// `classes` counts it too.
	Interface bool
	// FieldCount is how many fields the type declares, and 0 for a type that is not a struct.
	//
	// An embedded field counts as one, because that is what the declaration adds here; how many fields
	// the embedded type brings with it is a question about another declaration. A grouped `a, b int` is
	// two.
	FieldCount int
	// MethodCount is how many methods the type has: for an interface the members it lists, and for every
	// other type the functions declared with it as their receiver, pointer and value receivers alike.
	//
	// A method is declared beside its type rather than inside it, so this is counted across the files of
	// the type's own folder — which are the files the rule selected, as ExtractFileInfo describes. An
	// interface's members are its own declaration and need no such search.
	MethodCount int
	// Fields are the fields the type declares, in the order they were declared, each with the methods that
	// reach it. It is FieldCount's population by name — there is one entry per field that count counts — and
	// it is empty for a type that is not a struct.
	Fields []FieldInfo
	// Methods are the methods declared with this type as their receiver, in the order they were declared,
	// each with the fields of this class it reaches. Together with Fields it is what every cohesion metric is
	// calculated from.
	//
	// An interface's members are not among them, though MethodCount counts them: a member has no body, so
	// there is no field for it to reach and no cohesion for a formula to read off it. Neither is a function
	// declared without a receiver — that is the file's, and FileInfo.FunctionCount counts it.
	Methods []MethodInfo
}

// extractClassInfo describes the types this file declares, in the order they were declared. A grouped
// `type ( A ...; B ... )` declaration is as many classes as it has specs, because a rule about a type
// should not care how the declarations were bracketed.
func extractClassInfo(identifier, directory string, file *ast.File) []ClassInfo {
	var classes []ClassInfo
	for _, declaration := range file.Decls {
		declared, isDeclaration := declaration.(*ast.GenDecl)
		if !isDeclaration || declared.Tok != token.TYPE {
			continue
		}
		for _, spec := range declared.Specs {
			if declaredType, isType := spec.(*ast.TypeSpec); isType {
				classes = append(classes, newClassInfo(identifier, directory, declaredType))
			}
		}
	}
	return classes
}

// newClassInfo describes one declared type: its names, its fields, and whichever of the two counts its
// declaration answers on its own. A type that is neither a struct nor an interface — `type ID string`, a
// function type, an alias — has no fields and no members of its own, and gets its methods from
// collectReceiverMethods like a struct does.
func newClassInfo(identifier, directory string, declared *ast.TypeSpec) ClassInfo {
	class := ClassInfo{
		Name:       declared.Name.Name,
		Identifier: qualify(directory, declared.Name.Name),
		Path:       identifier,
	}
	switch underlying := declared.Type.(type) {
	case *ast.StructType:
		class.Fields = declaredFields(underlying.Fields)
		class.FieldCount = len(class.Fields)
	case *ast.InterfaceType:
		class.Interface = true
		class.MethodCount = countFields(underlying.Methods)
	}
	return class
}

// qualify spells the identifier of a declared type: the folder it was declared in, a dot, and the name —
// `internal/api.Handler`. That is the shape matching.TargetClassname strips a bare name out of.
//
// A type declared at the project root is its bare name. The root's folder identifier is `.`, and
// `..Server` would be a stranger spelling of `Server` than the name itself — while still matching every
// pattern the name does, because the qualification is stripped before matching either way.
func qualify(directory, name string) string {
	if directory == "" || directory == "." {
		return name
	}
	return directory + "." + name
}

// countFields counts the members an interface's field list lists: one per name, and one for an entry that has
// no name.
//
// The nameless entry is the embedded one — an interface, or an element of a type constraint — and counting it
// as one is the same reading in both directions: the declaration adds one member here, and what the embedded
// type brings with it is a count of another declaration, which this package would have to resolve a type to
// know. A struct's fields are counted by declaredFields instead, which has to name each of them anyway.
func countFields(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}
	count := 0
	for _, field := range fields.List {
		count += max(len(field.Names), 1)
	}
	return count
}

// declaredFields describes the fields a struct declares, in the order they were declared: one per name, so a
// grouped `a, b int` is two, and one for the nameless entry that is an embedded field.
//
// It is what FieldCount counts, named rather than tallied, because a field is only half of what a cohesion
// metric needs: the other half is which methods reach it, and a method reaches it by the name in here.
func declaredFields(fields *ast.FieldList) []FieldInfo {
	if fields == nil {
		return nil
	}
	declared := make([]FieldInfo, 0, len(fields.List))
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			declared = append(declared, FieldInfo{Name: baseTypeName(field.Type)})
			continue
		}
		for _, name := range field.Names {
			declared = append(declared, FieldInfo{Name: name.Name})
		}
	}
	return declared
}

// receiver is a type methods can be declared on: the folder the declarations live in, and the name of the
// type. The folder is half of the key because a method travels with its package rather than its file, and
// two packages may both declare a `Handler`.
type receiver struct {
	directory string
	name      string
}

// declaredMethod is one method declaration as the folder's tally holds it: the name it was declared with, and
// the names it selected on its receiver.
//
// The selected names are unfiltered on purpose. Which of them are fields of the class is attributeMethods'
// question, because the answer needs the type's own declaration and that may be in another file of the folder
// — the same reason the methods are tallied per folder rather than counted per file.
type declaredMethod struct {
	// name is the declared name of the method — `Handle`, of `func (h *Handler) Handle()`.
	name string
	// selected are the names this method selects on its receiver, in the order it first selects each of them
	// and each of them once.
	selected []string
}

// collectReceiverMethods adds this file's method declarations to a tally per receiver type. A declaration
// whose receiver names no type — which the parser accepts and the compiler does not — is left out rather than
// attributed to a nameless class.
func collectReceiverMethods(methods map[receiver][]declaredMethod, directory string, file *ast.File) {
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Recv == nil {
			continue
		}
		if name := receiverTypeName(function.Recv); name != "" {
			key := receiver{directory: directory, name: name}
			methods[key] = append(methods[key], declaredMethod{
				name:     function.Name.Name,
				selected: selectedOnReceiver(receiverName(function.Recv), function.Body),
			})
		}
	}
}

// receiverTypeName is the name of the type a method is declared on, with the pointer star and the type
// parameters of a generic receiver stripped: `Stack` of both `func (s Stack) ...` and
// `func (s *Stack[T]) ...`. A method belongs to the type, not to the way the receiver was spelled.
func receiverTypeName(receivers *ast.FieldList) string {
	if receivers == nil || len(receivers.List) == 0 {
		return ""
	}
	return baseTypeName(receivers.List[0].Type)
}

// receiverName is the variable a method calls its receiver — `h`, of `func (h *Handler) Handle()` — and the
// empty string when the declaration names none.
//
// A method that does not name its receiver, and one that names it `_`, cannot reach a field through it, so
// both read as no name at all: what the receiver is called is how an access to a field is recognized.
func receiverName(receivers *ast.FieldList) string {
	if receivers == nil || len(receivers.List) == 0 || len(receivers.List[0].Names) == 0 {
		return ""
	}
	if name := receivers.List[0].Names[0].Name; name != "_" {
		return name
	}
	return ""
}

// selectedOnReceiver are the names this method body selects on its receiver, in the order it first selects
// each of them and each of them once: `name` and `size`, of a body holding `h.name`, `h.size` and `h.name`
// again.
//
// A selection at any depth counts, because `h.inner.deep` reaches `inner` on the way, and a selection on
// anything else does not: `other.name` is a field of another value. What the name means is left to the caller
// — a field of the class, a method of it, or something this package cannot see — and only a method with a body
// and a named receiver can select anything at all.
//
// A receiver shadowed inside the body would be read as the receiver here. Telling the two apart needs a
// resolved type, which this package deliberately does not have: it parses a file rather than type-checking a
// package, so a rule about a number never waits for the toolchain to build the project.
func selectedOnReceiver(name string, body *ast.BlockStmt) []string {
	if name == "" || body == nil {
		return nil
	}
	var selected []string
	ast.Inspect(body, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		if base, isIdent := selector.X.(*ast.Ident); isIdent && base.Name == name && !slices.Contains(selected, selector.Sel.Name) {
			selected = append(selected, selector.Sel.Name)
		}
		return true
	})
	return selected
}

// baseTypeName unwraps a type expression down to the declared name it names, and is empty for anything else.
//
// It is what a method's receiver names — `Stack`, of `*Stack[T]` — and what a method selects an embedded field
// by: the package qualifier of an embedded `io.Reader` is not part of the name a method reaches it through,
// which is `Reader`.
func baseTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return baseTypeName(typed.X)
	case *ast.IndexExpr:
		return baseTypeName(typed.X)
	case *ast.IndexListExpr:
		return baseTypeName(typed.X)
	case *ast.SelectorExpr:
		return typed.Sel.Name
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

// attributeMethods gives every class the methods its folder declared on it — on top of the members its own
// declaration listed — and the fields each of those methods reaches. It is the second pass of describeSources,
// and it mutates the classes in place because they are the slices this package built moments ago.
//
// A tallied receiver naming a type none of these files declares adds nothing: it is either a type in a
// file the scope left out, or one this library has no class for.
func attributeMethods(files []FileInfo, methods map[receiver][]declaredMethod) {
	for fileIndex := range files {
		for classIndex := range files[fileIndex].Classes {
			class := &files[fileIndex].Classes[classIndex]
			declared := methods[receiver{directory: files[fileIndex].Directory, name: class.Name}]
			class.MethodCount += len(declared)
			attributeAccesses(class, declared)
		}
	}
}

// attributeAccesses records which of this class's fields each of these methods reaches, in both directions at
// once: the method keeps the fields it selected, and every field keeps the methods that selected it.
//
// The two are written here together so that they are one fact rather than two that could disagree. A name a
// method selected on its receiver that is not a field of this class is dropped — it is a method of it, a field
// promoted from an embedded type, or something a resolved type would be needed to name — because a cohesion
// metric counts accesses to the fields the class declares.
func attributeAccesses(class *ClassInfo, declared []declaredMethod) {
	if len(declared) == 0 {
		return
	}
	position := make(map[string]int, len(class.Fields))
	for index, field := range class.Fields {
		position[field.Name] = index
	}

	class.Methods = make([]MethodInfo, 0, len(declared))
	for _, method := range declared {
		accessed := make([]string, 0, len(method.selected))
		for _, name := range method.selected {
			index, isField := position[name]
			if !isField {
				continue
			}
			accessed = append(accessed, name)
			class.Fields[index].AccessedBy = append(class.Fields[index].AccessedBy, method.name)
		}
		class.Methods = append(class.Methods, MethodInfo{Name: method.name, AccessedFields: accessed})
	}
}
