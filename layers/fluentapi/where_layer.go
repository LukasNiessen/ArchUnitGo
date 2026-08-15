package fluentapi

// LayerPolicyBuilder is a clause that has named its layer and not yet said anything about it: the stage
// between `where layer("api")` and the `may ... depend on layers` that closes the sentence.
//
// It is the scope of a clause, and it is a stage of its own for the same reason LayerBuilder is: a layer
// named with nothing said about it is not a rule, so the type system asks for the predicate here rather than
// handing back something checkable that judges nothing.
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("api").MayOnlyDependOnLayers("db").
//		WhereLayer("db").MayOnlyDependOnLayers()
//	violations, err := rule.Check(nil)
//
// There is no mood stage between this one and the predicate. The two predicates are the two moods — `may
// only depend on layers` is the allowlist and `may not depend on layers` the blocklist — so `should` would
// have nothing left to say; the mood itself still travels on the clause, as assertion.Clause explains.
type LayerPolicyBuilder struct {
	// policy is the policy this clause will join: every layer declared so far and every clause already
	// written, which is what makes a chain of clauses one rule.
	policy LayersBuilder
	// layer is the name of the layer the clause is about — the depending end of every dependency it judges.
	layer string
}

// WhereLayer opens a clause about one of the policy's layers: `where layer("api")`.
//
// It is the scope of a clause and it names a layer that was declared with `layer`, earlier in the same policy,
// by exactly the name it was declared under — so a policy declares who exists and then says what they may do,
// in that order. A name the policy has not declared is reported by the terminal as a UserError wrapping
// ErrUndeclaredLayer, because an undeclared layer has no files and a clause about it would judge nothing at
// all.
//
// A policy may have as many clauses as it needs, about one layer or about several, and all of them are in
// force at once — which is what makes an N-layer policy readable: one line per layer, each saying what that
// layer is allowed to reach.
//
// The empty string is not a layer and is reported as a UserError wrapping ErrUnnamedLayer.
func (b LayersBuilder) WhereLayer(name string) LayerPolicyBuilder {
	if name == "" {
		return LayerPolicyBuilder{policy: b.rejecting("where layer", name, ErrUnnamedLayer), layer: name}
	}
	if !b.declares(name) {
		return LayerPolicyBuilder{policy: b.rejecting("where layer", name, ErrUndeclaredLayer), layer: name}
	}
	return LayerPolicyBuilder{policy: b, layer: name}
}

// WhereLayer opens the next clause of a policy that already has one, so that the clauses of a policy read as
// a list: `where layer "api", may only depend on layers "db", where layer "db", may only depend on no
// layers`.
//
// It is the same verb one stage later, and it is what makes a whole layer policy one chain — which is the
// point of this module, since the alternative is one rule per layer and a reader assembling the policy in
// their head. The clause already written is kept: every clause of a policy applies.
//
// There is no `layer` verb here beside it, deliberately: a policy declares its layers and then says what they
// may do, so every clause can be read against a complete set of declarations and a name that is not one of
// them is a typo the chain can reject where it was typed.
func (c LayersPolicyCondition) WhereLayer(name string) LayerPolicyBuilder {
	return c.policy.WhereLayer(name)
}
