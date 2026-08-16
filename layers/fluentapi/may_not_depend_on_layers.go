package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// MayNotDependOnLayers closes a clause with the blocklist: the layer it is about may not depend on any of the
// layers named here, and the rest of the policy's layers are left alone.
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("db").MayNotDependOnLayers("api")
//	violations, err := rule.Check(nil)
//
// It is the clause for the one edge a team cares about — the database that must not call back into the API,
// the domain that must not reach the transport — and for tightening a policy one direction at a time without
// having to enumerate everything the layer legitimately uses, which is what the allowlist would ask for.
//
// Blocklist clauses are asked before allowlist ones, which matters only when both kinds of clause are broken
// by the same dependency: the violation is then reported against this one, because it is the sentence the
// reader wrote about that very pair of layers. Both kinds are otherwise simply in force together.
//
// Naming no layer at all is rejected, and the terminal reports it as a UserError wrapping ErrNoLayersNamed: a
// blocklist that forbids nothing holds forever, which is the failure this library refuses to pass silently
// everywhere else too. The sealed layer is the other clause with no argument — MayOnlyDependOnLayers() — which
// forbids everything instead.
//
// Every layer named has to have been declared by `layer`, earlier in this policy, under exactly that name; a
// name the policy has not declared is reported as a UserError wrapping ErrUndeclaredLayer. The clause joins the
// policy and the result is checkable, and chainable for the next clause.
func (p LayerPolicyBuilder) MayNotDependOnLayers(names ...string) LayersPolicyCondition {
	policy := p.policy
	if len(names) == 0 {
		policy = policy.rejecting("may not depend on layers", p.layer, ErrNoLayersNamed)
	}
	// The clause joins the policy even when it was rejected, unlike a pattern that would not compile: it
	// renders as `may not depend on no layers` and the rejection after it, which is what a reader has to see
	// in order to recognize the clause they typed. The policy carries the error, so it will not be judged.
	return policy.clausing(p.layer, names, assertion.ShouldNot)
}
