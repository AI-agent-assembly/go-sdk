package assembly

// Drift gate binding the documented Go quick-start's enforcement claims to the
// controls that prove them (AAASM-5529, Epic AAASM-5526).
//
// docs/quick-start.md tells a reader what governance does for them: that a nil
// client denies under the default posture, that each governed call is checked
// before execution, and that a deny surfaces as a *assembly.PolicyViolationError
// with the inner tool never running. Nothing connected those sentences to the
// controls in quickstart_negative_control_test.go, so a claim could be added,
// reworded, or left standing after the behaviour beneath it changed and no gate
// would notice.
//
// WHAT THIS GATE PROVES
//
//  1. Every enforcement claim in the gated sections is bound to a named control.
//     The claim text is read out of the document, so a new claim that no binding
//     quotes fails here rather than shipping unbacked.
//  2. Every binding still describes the document. Rewording a claim breaks its
//     quote and fails.
//  3. Every control a binding names still exists. Control names are extracted
//     from the negative-control file's AST — including t.Run subtest names — so
//     renaming or deleting one fails here rather than leaving a binding pointing
//     at nothing.
//  4. The error type the document names is the one the SDK actually returns.
//     Derived by driving a real deny through WrapTools and reading
//     reflect.TypeOf on the returned error, NOT by naming the type in this file.
//     Naming it would make a rename a compile error, which is red but aborts
//     before the assertion meant to catch it can run — the inverted-order defect
//     the round-1 review of this ticket found in all three SDKs.
//
// WHAT THIS GATE DOES NOT PROVE
//
// It does not compile or execute a documented snippet. metadata/quickstart/
// holds .go.txt fragments precisely so the toolchain never sees them
// (metadata/quickstart/README.md), and the docs-metadata workflow round-trips
// them as text: it proves the generated tabs match the vendored fragments and
// nothing more. This gate does not change that.
//
// It also does not make a claim true by binding it. The "Putting it together"
// program is registered as unproven and names AAASM-5662, which measured that
// the program exits 1 as written.

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	quickStartDoc        = "../docs/quick-start.md"
	negativeControlFile  = "quickstart_negative_control_test.go"
	claimKindEnforcement = "enforcement"
	claimKindLifecycle   = "lifecycle"
)

// claimBinding is one documented claim and the controls that stand behind it.
type claimBinding struct {
	id   string
	kind string
	// quote is a verbatim fragment of the claim as it appears in the document,
	// with Markdown's soft wrapping collapsed. Rewording the document breaks it.
	quote string
	// controls are "TestName/SubtestName" ids in the negative-control file.
	controls []string
	// unprovenReason is set when no control proves the claim. It must name a
	// ticket, which the gate below enforces.
	unprovenReason string
}

var quickStartClaimBindings = []claimBinding{
	{
		id:    "nil-client-denies-under-default-posture",
		kind:  claimKindEnforcement,
		quote: "passing `nil` denies every wrapped call",
		controls: []string{
			"TestQuickStartDegradedPathCannotLookProtected/NilClientUnderTheDefaultPostureDeniesRatherThanRunning",
		},
	},
	{
		id:    "fail-open-is-a-true-passthrough",
		kind:  claimKindEnforcement,
		quote: "for a true passthrough wrapper (the tools run,",
		// The opt-out needs its own control: without it, the deny above is
		// indistinguishable from "the wrapper never runs anything".
		controls: []string{
			"TestQuickStartDegradedPathCannotLookProtected/NilClientWithFailOpenRunsTheToolAndSaysSo",
		},
	},
	{
		id:    "governed-call-checked-before-execution",
		kind:  claimKindEnforcement,
		quote: "is checked against the gateway policy before execution",
		// Both halves are named. The negative controls prove the "before" by the
		// absence of the side effect; the positive controls prove the probe
		// would have seen that effect had it happened. Either alone is the
		// vacuous evidence this Epic exists to remove.
		controls: []string{
			"TestQuickStartFilesystemNegativeControl/PositiveControl_AllowedWriteCreatesTheFile",
			"TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			"TestQuickStartNetworkNegativeControl/PositiveControl_AllowedEgressReachesTheListener",
			"TestQuickStartNetworkNegativeControl/NegativeControl_DeniedEgressNeverReachesTheListener",
		},
	},
	{
		id:    "outcome-offered-to-record-result",
		kind:  claimKindEnforcement,
		quote: "its outcome is offered to `RecordResult` after",
		controls: []string{
			"TestQuickStartDenyIsAttributable/TheDeniedCallEmitsAnAuditRecordNamingTheToolAndRun",
			"TestQuickStartDenyIsAttributable/AnAllowedCallIsAuditedUnderTheSameRunID",
		},
	},
	{
		id:   "shipped-client-discards-the-record",
		kind: claimKindEnforcement,
		// A negative capability claim, and the honest one: it says the SDK layer
		// keeps no audit trail. It is bound so that if someone later deletes the
		// caveat because a sink was wired (AAASM-5750), this gate makes them
		// revisit the control rather than quietly dropping the sentence.
		quote: "discards that record rather than retaining it",
		controls: []string{
			"TestQuickStartDenyIsAttributable/TheDeniedCallEmitsAnAuditRecordNamingTheToolAndRun",
		},
	},
	{
		id:    "deny-surfaces-as-policy-violation-and-tool-never-runs",
		kind:  claimKindEnforcement,
		quote: "and the inner tool never runs",
		controls: []string{
			"TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			"TestQuickStartNetworkNegativeControl/NegativeControl_DeniedEgressNeverReachesTheListener",
			"TestQuickStartWrappingDoesNotDisarmTheOriginalTool",
		},
	},
	{
		id:    "init-succeeds-once-a-gateway-is-reachable",
		kind:  claimKindLifecycle,
		quote: "once a gateway is reachable (resolved or auto-started)",
		unprovenReason: "AAASM-5662: the documented program was executed and exits 1 — " +
			"Init fails before any tool call. No control covers the documented " +
			"program, and binding one of the WrapTools controls to it would " +
			"misrepresent what was measured.",
	},
}

// fencedCodeBlock matches ```...``` spans.
var fencedCodeBlock = regexp.MustCompile("(?s)```.*?```")

// flattenMarkdown drops fenced code and collapses soft wrapping, so a quote can
// span wrapped lines and a code sample is never mistaken for a prose claim.
func flattenMarkdown(text string) string {
	return strings.Join(strings.Fields(fencedCodeBlock.ReplaceAllString(text, " ")), " ")
}

// nextHeading finds the next Markdown heading of any level. Ending a region at
// "\n## " alone ran the WrapTools prose past "### Govern your first agent" and
// swept three unrelated sentences into the claim scan.
var nextHeading = regexp.MustCompile(`(?m)^#{1,6} `)

// gatedDocumentClaims returns the flattened text of the quick-start regions this
// gate is responsible for, keyed by region name.
//
// Read from the document rather than transcribed, so a claim added to either
// region shows up here without anyone editing this file.
func gatedDocumentClaims(t *testing.T) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(quickStartDoc)
	if err != nil {
		t.Fatalf("cannot read %s: %v", quickStartDoc, err)
	}
	document := string(raw)

	regions := map[string]string{
		// The WrapTools prose, from the sentence after the fenced example to the
		// next heading of any level.
		"wrap-tools-prose": "The second argument is the `GovernanceClient`",
		"what-to-expect":   "### What to expect",
	}

	claims := make(map[string]string, len(regions))
	for name, opening := range regions {
		start := strings.Index(document, opening)
		if start == -1 {
			t.Fatalf(
				"%s no longer contains the %q region (looked for %q).\n"+
					"If the quick-start was restructured, re-point this gate at the section that "+
					"now carries the enforcement claims — do not delete it.",
				quickStartDoc, name, opening,
			)
		}
		// Skip the region's own opening LINE before searching for the next
		// heading. Advancing a single character is not enough: with (?m) the
		// pattern's ^ also matches at the start of the slice, so the region
		// collapsed to its own first character.
		body := document[start:]
		searchFrom := strings.Index(body, "\n")
		if searchFrom == -1 {
			searchFrom = len(body)
		}
		if offset := nextHeading.FindStringIndex(body[searchFrom:]); offset != nil {
			body = body[:searchFrom+offset[0]]
		}
		claims[name] = flattenMarkdown(body)
	}
	return claims
}

// controlNodeIDs extracts "TestName/SubtestName" ids from the negative-control
// file's AST, including the t.Run subtest literals.
//
// Derived from the source rather than transcribed, so this set changes when a
// control is renamed or removed and the bindings above then fail.
func controlNodeIDs(t *testing.T) map[string]bool {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, negativeControlFile, nil, 0)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", negativeControlFile, err)
	}

	ids := make(map[string]bool)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		testName := fn.Name.Name
		ids[testName] = true

		// go test addresses a subtest as Parent/Sub, with spaces replaced by
		// underscores. Collect the t.Run name literals to match that form.
		ast.Inspect(fn, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Run" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			subtest, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			ids[testName+"/"+strings.ReplaceAll(subtest, " ", "_")] = true
			return true
		})
	}
	return ids
}

// TestClaimGateCanSeeWhatItGates is the positive control for the gate itself.
// Every check below reads a real artifact; these prove the reads arrived. An
// empty parse and a clean result are otherwise indistinguishable.
func TestClaimGateCanSeeWhatItGates(t *testing.T) {
	t.Run("TheGatedDocumentRegionsAreFoundAndNonEmpty", func(t *testing.T) {
		claims := gatedDocumentClaims(t)
		if len(claims) != 2 {
			t.Fatalf("parsed %d regions, want 2: %v", len(claims), claims)
		}
		for name, text := range claims {
			if len(text) < 80 {
				t.Fatalf("region %q is only %d chars — too short to hold its claims: %q", name, len(text), text)
			}
		}
	})

	t.Run("TheASTExtractionFindsTheNegativeControls", func(t *testing.T) {
		ids := controlNodeIDs(t)
		if len(ids) < 12 {
			t.Fatalf("AST extraction found only %d control ids, want at least 12: %v", len(ids), ids)
		}
		// A named one, so an extraction that returned an unrelated set of the
		// right size cannot satisfy the count above.
		const known = "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile"
		if !ids[known] {
			t.Fatalf("AST extraction did not find %q; it found %v", known, ids)
		}
	})
}

// TestEveryDocumentedClaimIsBound is the check that makes this gate load-bearing
// rather than decorative: a new enforcement sentence cannot reach the published
// quick-start without someone naming the control behind it.
func TestEveryDocumentedClaimIsBound(t *testing.T) {
	t.Run("EveryBindingStillQuotesTheDocument", func(t *testing.T) {
		claims := gatedDocumentClaims(t)
		for _, binding := range quickStartClaimBindings {
			found := false
			for _, text := range claims {
				if strings.Contains(text, binding.quote) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf(
					"claim binding %q quotes:\n  %q\nwhich no longer appears in the gated regions of %s.\n"+
						"The claim was reworded or removed. Update the quote and re-check that the "+
						"named controls still prove the new wording.",
					binding.id, binding.quote, quickStartDoc,
				)
			}
		}
	})

	t.Run("NoSentenceInTheGatedRegionsIsUnbound", func(t *testing.T) {
		// Split each region into sentences and require every sentence that makes
		// an enforcement claim to be covered by a binding. Prose that merely
		// points elsewhere is not a claim.
		// Deliberately excludes a bare "governed": it is the variable name the
		// documented sample uses ("Hand `governed` to your agent"), so matching
		// it swept prose that makes no enforcement claim into the scan.
		enforcementLanguage := regexp.MustCompile(
			`(?i)\bdenie[sd]\b|\bdeny\b|\bblocked\b|\bnever runs\b|\bbefore execution\b` +
				`|\bchecked against\b|\benforce[sd]?\b|\bpassthrough\b|\bdiscards\b`,
		)
		sentenceSplit := regexp.MustCompile(`(?m)\.\s`)

		for region, text := range gatedDocumentClaims(t) {
			for _, sentence := range sentenceSplit.Split(text, -1) {
				sentence = strings.TrimSpace(sentence)
				if sentence == "" || !enforcementLanguage.MatchString(sentence) {
					continue
				}
				bound := false
				for _, binding := range quickStartClaimBindings {
					if strings.Contains(sentence, binding.quote) {
						bound = true
						break
					}
				}
				if !bound {
					t.Errorf(
						"this enforcement sentence in region %q has no claimBinding:\n  %q\n\n"+
							"Add a claimBinding naming the control that proves it. If no control "+
							"does, set unprovenReason and name the ticket — do not delete the "+
							"claim from this gate to make it pass.",
						region, sentence,
					)
				}
			}
		}
	})
}

// TestEveryBindingNamesSomethingReal fails when a control a binding depends on
// is renamed or removed.
func TestEveryBindingNamesSomethingReal(t *testing.T) {
	t.Run("NamedControlsExist", func(t *testing.T) {
		available := controlNodeIDs(t)
		for _, binding := range quickStartClaimBindings {
			for _, control := range binding.controls {
				if !available[control] {
					t.Errorf(
						"claim binding %q names a control that does not exist in %s:\n  %s\n\n"+
							"The control was renamed or removed. Re-point the binding at the control "+
							"that now proves the claim, or mark the claim unproven and name the ticket.",
						binding.id, negativeControlFile, control,
					)
				}
			}
		}
	})

	t.Run("AnEnforcementClaimIsEitherProvenOrOpenlyUnproven", func(t *testing.T) {
		ticketPattern := regexp.MustCompile(`AAASM-\d+`)
		for _, binding := range quickStartClaimBindings {
			if len(binding.controls) > 0 {
				continue
			}
			if binding.unprovenReason == "" {
				t.Errorf(
					"claim %q names no control and gives no unprovenReason. One or the other "+
						"is required: a documented enforcement claim with neither is exactly the "+
						"unbacked assertion AAASM-5526 exists to eliminate.",
					binding.id,
				)
				continue
			}
			if !ticketPattern.MatchString(binding.unprovenReason) {
				t.Errorf(
					"claim %q is unproven but its reason names no ticket. An unproven claim "+
						"must be traceable to the work that resolves it. Reason given: %q",
					binding.id, binding.unprovenReason,
				)
			}
		}
	})
}

// TestTheDocumentedErrorTypeIsTheOneTheSDKReturns drives a real deny through the
// exported WrapTools API and reads the concrete type name off the returned
// error with reflection.
//
// The type is deliberately NOT named in this file. Naming it would make a rename
// a compile error — red, but it aborts the package before this assertion can
// run, leaving the check that is supposed to catch the rename unexercised. That
// is the inverted-order defect the round-1 review found in all three SDKs, and
// it is invisible unless you mutate and watch which line fails.
func TestTheDocumentedErrorTypeIsTheOneTheSDKReturns(t *testing.T) {
	tool := newFileSideEffectTool(t)
	client := newPolicyGovernanceClient(map[string]string{"write_to_disk": "policy forbids disk writes"})

	governed := WrapTools([]Tool{tool}, client)
	_, err := governed[0].Call(WithAgentID(context.Background(), "claim-binding-agent"), "denied")

	// Absence of the effect first, as in every control in this package: an error
	// assertion placed ahead of it aborts before the side effect is checked.
	if tool.occurred() {
		t.Fatal("the governed call ran the tool body; this gate is measuring the wrong path")
	}
	if err == nil {
		t.Fatal("governed call was expected to be denied")
	}

	// The name as the running SDK reports it, derived not transcribed.
	actual := reflect.TypeOf(err).String()

	// The name as the document promises a reader. Read out of the document, so
	// changing either side without the other fails here.
	document, readErr := os.ReadFile(quickStartDoc)
	if readErr != nil {
		t.Fatalf("cannot read %s: %v", quickStartDoc, readErr)
	}
	documented := regexp.MustCompile(`\*assembly\.[A-Za-z0-9_]+Error`).FindAllString(string(document), -1)
	if len(documented) == 0 {
		t.Fatalf(
			"%s no longer names any *assembly.<Name>Error type. The quick-start used to "+
				"promise a reader the concrete error a deny surfaces as; if that promise was "+
				"removed, this gate must be re-pointed rather than deleted.",
			quickStartDoc,
		)
	}

	found := false
	for _, name := range documented {
		if name == actual {
			found = true
			break
		}
	}
	if !found {
		t.Errorf(
			"a denied call returns %s, but %s names only %v.\n"+
				"Either the type was renamed and the documentation now points at something a "+
				"reader cannot find, or the deny path changed which error it returns.",
			actual, filepath.Base(quickStartDoc), documented,
		)
	}
}

// TestTheDocumentedSentinelIsTheOneTheNilClientReturns pins the other named
// error in the WrapTools prose.
//
// Unlike the type above, a sentinel's identifier cannot be recovered from a
// value, so this one references ErrGovernanceUnavailable directly and is
// therefore COMPILE-gated, not assertion-gated: renaming it breaks the build
// rather than producing the message below. That is a weaker signal and is called
// out here so nobody reads this test as proving more than it does.
func TestTheDocumentedSentinelIsTheOneTheNilClientReturns(t *testing.T) {
	tool := newFileSideEffectTool(t)

	governed := WrapTools([]Tool{tool}, nil)
	_, err := governed[0].Call(WithAgentID(context.Background(), "claim-binding-agent"), "degraded")

	if tool.occurred() {
		t.Fatalf("no governance client was available, yet %s was written", tool.path)
	}
	if !errors.Is(err, ErrGovernanceUnavailable) {
		t.Fatalf("err = %v, want ErrGovernanceUnavailable", err)
	}

	document, readErr := os.ReadFile(quickStartDoc)
	if readErr != nil {
		t.Fatalf("cannot read %s: %v", quickStartDoc, readErr)
	}
	if !strings.Contains(string(document), "ErrGovernanceUnavailable") {
		t.Errorf(
			"the nil-client deny path returns ErrGovernanceUnavailable, but %s no longer "+
				"names it. A reader is told the call is denied without being told what to "+
				"match on.",
			filepath.Base(quickStartDoc),
		)
	}
}
