package assembly

// Drift gate binding the documented Go quick-start's enforcement claims to the
// controls that prove them (AAASM-5529, Epic AAASM-5526).
//
// docs/quick-start.md tells a reader what governance does for them. Those
// sentences are the product's load-bearing enforcement claims, and nothing
// connected them to the controls that prove them. A claim could be added,
// reworded, or left standing after the behaviour beneath it changed, and no
// gate would notice.
//
// WHAT THIS GATE PROVES
//
//  1. The WHOLE document is scanned, not an opted-in region. Every sentence
//     using enforcement vocabulary must be bound. Sections and sentences may be
//     excluded only through the two named allow-lists below, each entry
//     carrying a reason and an exact sentence, so an allow-list entry cannot
//     cover a reworded or newly added claim.
//  2. A binding must match a WHOLE sentence, exactly — compared with ==, never
//     with strings.Contains. Containment let a sentence carry unlimited extra
//     unbound claims, up to and including its own negation, as long as one
//     bound fragment survived.
//  3. Exactly one binding may match a sentence, so two bindings cannot split
//     responsibility for one claim and leave neither owning it.
//  4. Every control a binding names still exists. Control ids are extracted
//     from each control file's AST as "<file> :: TestName/Subtest".
//  5. Every claim is proven or openly unproven, with no exempt category. There
//     was a `kind` field here; it was written and never read, which is worse
//     than a bypass because a reader assumes it is enforced. Removed.
//  6. The error type and the sentinel the document names are the ones the SDK
//     actually produces, both DERIVED rather than transcribed.
//
// WHAT THIS GATE DOES NOT PROVE
//
// It does not compile or execute a documented snippet. metadata/quickstart/
// holds .go.txt fragments precisely so the toolchain never sees them
// (metadata/quickstart/README.md), and the docs-metadata generator check
// round-trips them as text. This gate does not change that.
//
// Binding a claim also does not make it true; where no control stands behind a
// sentence the binding says so and names a ticket.

import (
	"context"
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

const quickStartDoc = "../docs/quick-start.md"

// governanceErrorsFile is AST-parsed to recover the NAME of the sentinel the
// nil-client deny path returns. See TestTheDocumentedSentinelIsTheOneTheNilClientReturns.
const governanceErrorsFile = "governance_errors.go"

// controlFiles are the test files a binding may name.
//
// More than one, because the quick-start's claims are not all proved in the
// same place: the deny claims are proved by the negative controls, while the
// "discards that record" claim is proved by the audit-sink suite, which drives
// the SHIPPED client. Binding that claim to a negative control was a defect —
// those controls use a fixture client that retains the record, so the control
// stayed green when the shipped client's disposition flipped.
var controlFiles = []string{
	"quickstart_negative_control_test.go",
	"audit_sink_test.go",
}

// enforcementVocabulary marks a sentence as making a claim about what
// governance does. Deliberately wide: a narrow vocabulary is itself a bypass,
// because a new enforcement paragraph phrased around it is not treated as a
// claim at all.
var enforcementVocabulary = regexp.MustCompile(
	`(?i)\bdenie[sd]\b|\bdeny\b|\bblocked\b|\bblocking\b|\bnever runs?\b` +
		`|\bbefore execution\b|\bchecked against\b|\benforces?\b|\benforced\b` +
		`|\bpassthrough\b|\bdiscards?\b|\bdiscarded\b|\bthrows?\b|\brejects?\b` +
		`|\brouted\b|\bintercepts?\b|\binterception\b|\bgoverned\b|\bverified\b` +
		`|\bprotection\b|\bunprotected\b|\bbypass(ed|es)?\b`,
)

// excludedSections are whole sections skipped by the scan, each with a reason.
var excludedSections = map[string]string{
	"## Where to next": "A link list. Every line is a cross-reference; the claims live on the " +
		"pages linked to and are gated there.",
}

// excludedSentences are individual sentences skipped by the scan, each with a
// reason. Exact flattened sentences, never patterns, so an entry cannot
// silently cover a reworded or newly added claim — changing the sentence makes
// the entry stale and the allow-list test fails.
var excludedSentences = map[string]string{
	"Hand `governed` to your agent in place of the originals.": "Names the sample's variable. It " +
		"makes no capability claim; it matches the vocabulary only because the variable is called " +
		"`governed`.",
	"Two more validated Go examples already exist — **Tool Policy** and **CLI Runtime (sidecar)** — " +
		"but those are patterns (an allow/deny policy demo and sidecar wiring), not \"first agent\" " +
		"frameworks, so they're intentionally left out of this quick-start; see " +
		"`metadata/quickstart/README.md` for the tab-selection rationale.": "Editorial note about which " +
		"example tabs the page shows. It makes no claim about what governance does.",
}

// claimBinding is one documented claim and the controls that stand behind it.
type claimBinding struct {
	id string
	// quote is the claim as a WHOLE sentence, flattened. Compared with ==.
	quote string
	// controls are "<file> :: TestName/Subtest" ids drawn from controlFiles.
	controls []string
	// unprovenReason is set when no control proves the claim. Must name a ticket.
	unprovenReason string
}

const (
	negControl   = "quickstart_negative_control_test.go :: "
	auditControl = "audit_sink_test.go :: "
)

var quickStartClaimBindings = []claimBinding{
	{
		id: "walkthrough-checks-every-call",
		quote: "This walkthrough takes you from zero to a governed tool call in three steps: install " +
			"the SDK, initialise the runtime, and wrap your tools so every call is checked against the " +
			"AI Agent Assembly gateway.",
		controls: []string{
			negControl + "TestQuickStartFilesystemNegativeControl/PositiveControl_AllowedWriteCreatesTheFile",
			negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
		},
	},
	{
		id: "wraptools-governs-every-call",
		// Ends with a colon, not a period: the sentence introduces the fenced
		// sample that follows it. Before fenced blocks became paragraph breaks
		// this ran on into "The second argument is the `GovernanceClient`…",
		// and a binding quoting the glued pair would have covered two claims at
		// once — fragment containment one level up.
		quote: "`WrapTools` takes your `[]Tool` and a governance client, and returns a new `[]Tool` " +
			"where every `Call` is governed:",
		controls: []string{
			negControl + "TestQuickStartWrappingDoesNotDisarmTheOriginalTool",
		},
	},
	{
		id: "nil-client-denies-and-failopen-passes-through",
		quote: "Under the default fail-closed enforce posture, passing `nil` denies every wrapped call " +
			"(`ErrGovernanceUnavailable`) rather than running it unchecked — pass " +
			"`assembly.WithFailClosed(false)` for a true passthrough wrapper (the tools run, no " +
			"`Check`/`RecordResult` calls) while you wire in a real client, ready to enforce policy " +
			"(see [Handle allow/deny decisions and errors]({{< relref " +
			"\"/guides/handle-decisions-and-errors\" >}})).",
		// Both halves. The opt-out needs its own control: without it, the deny
		// is indistinguishable from "the wrapper never runs anything".
		controls: []string{
			negControl + "TestQuickStartDegradedPathCannotLookProtected/NilClientUnderTheDefaultPostureDeniesRatherThanRunning",
			negControl + "TestQuickStartDegradedPathCannotLookProtected/NilClientWithFailOpenRunsTheToolAndSaysSo",
		},
	},
	{
		id: "checked-before-execution-and-record-discarded",
		quote: "From here on, each call against a governed tool is checked against the gateway policy " +
			"before execution, and its outcome is offered to `RecordResult` after — though the client " +
			"this SDK ships discards that record rather than retaining it, so the SDK layer keeps no " +
			"audit trail of its own (see the warning on the [documentation home]({{< relref \"/\" >}}), " +
			"AAASM-5731).",
		// This sentence makes two claims, so it names controls for both.
		//
		// The "discards that record" half is bound to the audit-sink suite,
		// which drives the SHIPPED client end to end. It was previously bound to
		// a negative control, which was wrong in a way that matters: those
		// controls hand the record to a FIXTURE client that retains it, so
		// flipping the shipped client's disposition left them green. The control
		// named here is the one that goes red when the claim becomes false.
		controls: []string{
			negControl + "TestQuickStartFilesystemNegativeControl/PositiveControl_AllowedWriteCreatesTheFile",
			negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			auditControl + "TestShippedClientDeclaringNoRetentionReachesNothing",
			auditControl + "TestEveryShippedGovernanceClientDeclaresItsAuditSink",
		},
	},
	{
		id: "deny-surfaces-as-policy-violation-and-tool-never-runs",
		quote: "With a real governance client wired in, a `deny` decision surfaces as a " +
			"`*assembly.PolicyViolationError` and the inner tool never runs.",
		controls: []string{
			negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			negControl + "TestQuickStartNetworkNegativeControl/NegativeControl_DeniedEgressNeverReachesTheListener",
			negControl + "TestQuickStartWrappingDoesNotDisarmTheOriginalTool",
		},
	},
}

// yamlFrontMatter matches the Hugo front matter at the top of the document.
// Without stripping it, "title: Quick Start weight: 1" flattens into the first
// sentence and the binding for that sentence has to quote metadata.
var yamlFrontMatter = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)

// fencedCodeBlock matches ```...``` spans.
var fencedCodeBlock = regexp.MustCompile("(?s)```.*?```")

func flattenMarkdown(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// scannedSentences returns flattened sentence -> section heading for the whole
// document, minus the excluded sections.
//
// The gate opts sections OUT by name rather than opting them in, so a claim
// added to a section nobody thought about is still caught.
func scannedSentences(t *testing.T) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(quickStartDoc)
	if err != nil {
		t.Fatalf("cannot read %s: %v", quickStartDoc, err)
	}
	// Normalise line endings before anything else. Without this the paragraph
	// split never fires on a CRLF checkout: the whole section collapses into one
	// "sentence" that matches no binding. Node's four Windows CI legs caught
	// exactly that while Linux and macOS stayed green. This repo's CI is
	// Linux/macOS only, so the bug is latent here — normalised anyway, because a
	// gate whose result depends on the checkout's line endings is not a gate.
	body := strings.ReplaceAll(string(raw), "\r\n", "\n")
	body = yamlFrontMatter.ReplaceAllString(body, "")
	// A fenced block becomes a PARAGRAPH break, not a space. Replacing it with a
	// space glued the sentence before a code sample to the sentence after it,
	// and a binding quoting the glued pair would then cover two claims at once —
	// the same defect as fragment containment, one level up.
	body = fencedCodeBlock.ReplaceAllString(body, "\n\n")

	sentences := make(map[string]string)
	section := "(preamble)"
	headingLine := regexp.MustCompile(`(?m)^#{2,6} .*$`)

	offset := 0
	for _, loc := range append(headingLine.FindAllStringIndex(body, -1), []int{len(body), len(body)}) {
		chunk := body[offset:loc[0]]
		if _, skipped := excludedSections[section]; !skipped {
			for _, paragraph := range strings.Split(chunk, "\n\n") {
				for _, sentence := range splitSentences(paragraph) {
					if flat := flattenMarkdown(sentence); flat != "" {
						sentences[flat] = section
					}
				}
			}
		}
		if loc[1] > loc[0] {
			section = strings.TrimSpace(body[loc[0]:loc[1]])
		}
		offset = loc[1]
	}
	return sentences
}

// sentenceEnd splits on a period followed by whitespace.
var sentenceEnd = regexp.MustCompile(`(?s)\.\s`)

func splitSentences(paragraph string) []string {
	parts := sentenceEnd.Split(paragraph, -1)
	out := make([]string, 0, len(parts))
	for i, part := range parts {
		// Split consumed the terminator; put it back so a quote is a real sentence.
		if i < len(parts)-1 {
			part += "."
		}
		out = append(out, part)
	}
	return out
}

// claimSentences are the scanned sentences that make an enforcement claim.
func claimSentences(t *testing.T) map[string]string {
	t.Helper()
	claims := make(map[string]string)
	for sentence, section := range scannedSentences(t) {
		if !enforcementVocabulary.MatchString(sentence) {
			continue
		}
		if _, excluded := excludedSentences[sentence]; excluded {
			continue
		}
		claims[sentence] = section
	}
	return claims
}

// controlNodeIDs extracts "<file> :: TestName/Subtest" ids from every control
// file's AST, derived rather than transcribed.
func controlNodeIDs(t *testing.T) map[string]bool {
	t.Helper()

	ids := make(map[string]bool)
	for _, file := range controlFiles {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, file, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			ids[file+" :: "+fn.Name.Name] = true
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
				ids[file+" :: "+fn.Name.Name+"/"+strings.ReplaceAll(subtest, " ", "_")] = true
				return true
			})
		}
	}
	return ids
}

// sentinelNamesByMessage AST-parses governance_errors.go and returns
// message -> exported identifier for every `var ErrX = errors.New("...")`.
//
// This is what makes the sentinel check real. A rename tool renames the
// identifier but not a string literal in a test, so a check that hard-codes the
// name in a strings.Contains against the document passes while the SDK exports
// one name and the docs tell readers to match another. Deriving the name from
// source and looking it up by the message the running SDK actually returns
// closes that.
func sentinelNamesByMessage(t *testing.T) map[string]string {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, governanceErrorsFile, nil, 0)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", governanceErrorsFile, err)
	}

	byMessage := make(map[string]string)
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || len(spec.Values) != 1 {
			return true
		}
		name := spec.Names[0].Name
		if !strings.HasPrefix(name, "Err") {
			return true
		}
		call, ok := spec.Values[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		message, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		byMessage[message] = name
		return true
	})
	return byMessage
}

func readDocument(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(quickStartDoc)
	if err != nil {
		t.Fatalf("cannot read %s: %v", quickStartDoc, err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// TestClaimGateCanSeeWhatItGates is the positive control for the gate itself.
// An empty parse and a clean result are otherwise indistinguishable.
func TestClaimGateCanSeeWhatItGates(t *testing.T) {
	t.Run("TheWholeDocumentIsReadAndSplitIntoSentences", func(t *testing.T) {
		sentences := scannedSentences(t)
		if len(sentences) < 30 {
			t.Fatalf("only %d sentences parsed from the whole quick-start", len(sentences))
		}
	})

	t.Run("TheScanFindsEnforcementClaims", func(t *testing.T) {
		claims := claimSentences(t)
		if len(claims) < 5 {
			t.Fatalf("only %d claim sentences found: %v", len(claims), claims)
		}
	})

	t.Run("TheScanReachesMoreThanOneSection", func(t *testing.T) {
		// Narrowing the scan back to one region would otherwise look identical
		// to a clean pass.
		sections := make(map[string]bool)
		for _, section := range claimSentences(t) {
			sections[section] = true
		}
		if len(sections) < 3 {
			t.Fatalf("claims were found in only these sections: %v", sections)
		}
	})

	t.Run("TheASTExtractionFindsControlsInEveryControlFile", func(t *testing.T) {
		ids := controlNodeIDs(t)
		for _, file := range controlFiles {
			found := 0
			for id := range ids {
				if strings.HasPrefix(id, file+" :: ") {
					found++
				}
			}
			if found == 0 {
				t.Fatalf("AST extraction found no controls in %s", file)
			}
		}
		const known = negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile"
		if !ids[known] {
			t.Fatalf("AST extraction did not find %q", known)
		}
	})

	t.Run("TheSentinelExtractionFindsExportedErrorVars", func(t *testing.T) {
		byMessage := sentinelNamesByMessage(t)
		if len(byMessage) < 2 {
			t.Fatalf("AST extraction found %d sentinel vars in %s: %v",
				len(byMessage), governanceErrorsFile, byMessage)
		}
	})
}

// TestTheAllowListCannotBecomeABypass keeps the two exclusion lists from
// becoming the new hole.
func TestTheAllowListCannotBecomeABypass(t *testing.T) {
	t.Run("EveryExcludedSectionIsStillARealHeading", func(t *testing.T) {
		document := readDocument(t)
		for heading, reason := range excludedSections {
			if !strings.Contains(document, heading) {
				t.Errorf("excludedSections names %q, which is no longer a heading in %s. "+
					"A stale exclusion silently widens over time — remove it.", heading, quickStartDoc)
			}
			if strings.TrimSpace(reason) == "" {
				t.Errorf("exclusion %q carries no reason", heading)
			}
		}
	})

	t.Run("EveryExcludedSentenceIsStillPresentVerbatim", func(t *testing.T) {
		// An entry is a whole sentence, so rewording the claim makes the entry
		// stale and fails here rather than silently exempting the new wording.
		scanned := scannedSentences(t)
		for sentence, reason := range excludedSentences {
			if _, present := scanned[sentence]; !present {
				t.Errorf("excludedSentences contains a sentence that no longer appears in %s:\n  %q\n"+
					"It was reworded or removed. Delete the stale entry, and if the replacement makes "+
					"an enforcement claim, bind it.", quickStartDoc, sentence)
			}
			if strings.TrimSpace(reason) == "" {
				t.Errorf("exclusion of %q carries no reason", sentence)
			}
		}
	})
}

// TestEveryDocumentedClaimIsBound is what makes this gate load-bearing rather
// than decorative.
func TestEveryDocumentedClaimIsBound(t *testing.T) {
	t.Run("NoEnforcementSentenceIsUnbound", func(t *testing.T) {
		quotes := make(map[string]bool, len(quickStartClaimBindings))
		for _, binding := range quickStartClaimBindings {
			quotes[binding.quote] = true
		}
		for sentence, section := range claimSentences(t) {
			if quotes[sentence] {
				continue
			}
			t.Errorf("this sentence in section %q makes an enforcement claim and has no claimBinding:\n  %q\n\n"+
				"Add a claimBinding whose quote is the WHOLE sentence, naming the control that proves it. "+
				"If no control does, set unprovenReason and name the ticket. If the sentence genuinely "+
				"makes no capability claim, add it to excludedSentences with a reason — do not delete the "+
				"claim from this gate to make it pass.", section, sentence)
		}
	})

	t.Run("EachBindingMatchesExactlyOneWholeSentence", func(t *testing.T) {
		// Whole-sentence equality, not containment. Containment allowed a
		// sentence to carry extra unbound claims — up to and including its own
		// negation — while one bound fragment kept the gate green.
		scanned := scannedSentences(t)
		for _, binding := range quickStartClaimBindings {
			matches := 0
			for sentence := range scanned {
				if sentence == binding.quote {
					matches++
				}
			}
			if matches != 1 {
				t.Errorf("claim binding %q must match exactly one whole sentence in %s; it matched %d.\n"+
					"Its quote is:\n  %q\n"+
					"The claim was reworded, split, or merged. Update the quote to the new whole sentence "+
					"and re-check that the named controls still prove it.",
					binding.id, quickStartDoc, matches, binding.quote)
			}
		}
	})

	t.Run("NoTwoBindingsClaimTheSameSentence", func(t *testing.T) {
		seen := make(map[string]string)
		for _, binding := range quickStartClaimBindings {
			if other, duplicate := seen[binding.quote]; duplicate {
				t.Errorf("claim bindings %q and %q quote the same sentence. Split responsibility "+
					"like that and neither binding owns the claim.", other, binding.id)
			}
			seen[binding.quote] = binding.id
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
					t.Errorf("claim binding %q names a control that does not exist:\n  %s\n\n"+
						"The control was renamed or removed. Re-point the binding at the control that "+
						"now proves the claim, or mark the claim unproven and name the ticket.",
						binding.id, control)
				}
			}
		}
	})

	t.Run("AClaimIsEitherProvenOrOpenlyUnproven", func(t *testing.T) {
		// Every claim, with no exempt category. There used to be a `kind` field
		// here that was written and never read — dead decoration a reader would
		// assume was enforced. It was removed rather than wired up.
		ticketPattern := regexp.MustCompile(`AAASM-\d+`)
		for _, binding := range quickStartClaimBindings {
			if len(binding.controls) > 0 {
				continue
			}
			if binding.unprovenReason == "" {
				t.Errorf("claim %q names no control and gives no unprovenReason. One or the other is "+
					"required: a documented claim with neither is exactly the unbacked assertion "+
					"AAASM-5526 exists to eliminate.", binding.id)
				continue
			}
			if !ticketPattern.MatchString(binding.unprovenReason) {
				t.Errorf("claim %q is unproven but its reason names no ticket. Reason given: %q",
					binding.id, binding.unprovenReason)
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
// run, leaving the check that is supposed to catch the rename unexercised.
func TestTheDocumentedErrorTypeIsTheOneTheSDKReturns(t *testing.T) {
	tool := newFileSideEffectTool(t)
	client := newPolicyGovernanceClient(map[string]string{"write_to_disk": "policy forbids disk writes"})

	governed := WrapTools([]Tool{tool}, client)
	_, err := governed[0].Call(WithAgentID(context.Background(), "claim-binding-agent"), "denied")

	if tool.occurred() {
		t.Fatal("the governed call ran the tool body; this gate is measuring the wrong path")
	}
	if err == nil {
		t.Fatal("governed call was expected to be denied")
	}

	actual := reflect.TypeOf(err).String()

	documented := regexp.MustCompile(`\*assembly\.[A-Za-z0-9_]+Error`).FindAllString(readDocument(t), -1)
	if len(documented) == 0 {
		t.Fatalf("%s no longer names any *assembly.<Name>Error type. If that promise was removed, "+
			"this gate must be re-pointed rather than deleted.", quickStartDoc)
	}

	for _, name := range documented {
		if name == actual {
			return
		}
	}
	t.Errorf("a denied call returns %s, but %s names only %v.\n"+
		"Either the type was renamed and the documentation now points at something a reader cannot "+
		"find, or the deny path changed which error it returns.",
		actual, filepath.Base(quickStartDoc), documented)
}

// TestTheDocumentedSentinelIsTheOneTheNilClientReturns pins the sentinel the
// WrapTools prose tells a reader to match on.
//
// The identifier is DERIVED, not transcribed: the nil-client deny is driven for
// real, its message is looked up against the `var Err… = errors.New("…")` pairs
// AST-parsed out of governance_errors.go, and the resulting NAME is what the
// document must contain.
//
// The previous version hard-coded "ErrGovernanceUnavailable" in a
// strings.Contains against the document, which a rename tool leaves untouched
// while renaming the identifier — so the SDK could export one name and the docs
// tell readers to match another, with both green. That check was inert; this one
// is not.
func TestTheDocumentedSentinelIsTheOneTheNilClientReturns(t *testing.T) {
	tool := newFileSideEffectTool(t)

	governed := WrapTools([]Tool{tool}, nil)
	_, err := governed[0].Call(WithAgentID(context.Background(), "claim-binding-agent"), "degraded")

	// Absence of the effect first, as in every control in this package.
	if tool.occurred() {
		t.Fatalf("no governance client was available, yet %s was written", tool.path)
	}
	if err == nil {
		t.Fatal("nil-client call under the default posture was expected to be denied")
	}

	byMessage := sentinelNamesByMessage(t)
	derivedName := ""
	for message, name := range byMessage {
		if strings.Contains(err.Error(), message) {
			derivedName = name
			break
		}
	}
	if derivedName == "" {
		t.Fatalf("the nil-client deny returned %q, which matches no `var Err… = errors.New(…)` in %s.\n"+
			"Either the sentinel moved to another file or its message changed; re-point this check "+
			"rather than deleting it. Known sentinels: %v", err.Error(), governanceErrorsFile, byMessage)
	}

	if !strings.Contains(readDocument(t), derivedName) {
		t.Errorf("the nil-client deny path returns %s, but %s does not name it.\n"+
			"A reader is told the call is denied without being told what to match on — or the "+
			"identifier was renamed and the documentation still names the old one.",
			derivedName, filepath.Base(quickStartDoc))
	}
}
