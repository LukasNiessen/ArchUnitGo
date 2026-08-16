package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// MayOnlyDependOnLayers closes a clause with the allowlist: the layer it is about may depend on the layers
// named here and on no other declared layer.
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("domain").DefinedByFolder("internal/domain/**").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("api").MayOnlyDependOnLayers("domain", "db")
//	violations, err := rule.Check(nil)
//
// This is the clause a layered architecture is written as, and it is the reason this module exists: saying
// what a layer may reach is one line, while saying it as file rules is one rule per layer it may not reach.
// It is an allowlist, so a layer added to the policy later is forbidden to this one until the clause names it
// — which is the direction that keeps a policy honest as a project grows.
//
// Naming no layer at all is the sealed layer: `MayOnlyDependOnLayers()` allows nothing outside itself, so
// every dependency this layer has on another declared layer is a violation. It is a policy people really
// write — a domain that may not reach the code around it — and it is legal here, unlike the empty blocklist,
// which would forbid nothing and hold forever.
//
// It does not require any dependency to exist. The clause is broken by a dependency it did not name and by
// nothing else, so `may only depend on layers "domain"` says nothing about a layer that depends on nothing at
// all — that a layer's files exist is the empty-test guard's question, and it is asked of every declared
// layer.
//
// Every layer named has to have been declared by `layer`, earlier in this policy, under exactly that name; a
// name the policy has not declared is reported by the terminal as a UserError wrapping ErrUndeclaredLayer. The
// clause joins the policy and the result is checkable — and chainable, so the next clause follows straight on.
func (p LayerPolicyBuilder) MayOnlyDependOnLayers(names ...string) LayersPolicyCondition {
	return p.policy.clausing(p.layer, names, assertion.Should)
}
