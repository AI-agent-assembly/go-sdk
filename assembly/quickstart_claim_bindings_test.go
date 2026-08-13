package assembly

// Drift gate binding the documented Go quick-start's claims to the controls
// that prove them (AAASM-5529, Epic AAASM-5526).
//
// Every sentence in docs/quick-start.md must be either BOUND to a control that
// proves it, or explicitly ALLOW-LISTED as making no capability claim. There is
// no third state and no keyword filter.
//
// WHY THE DEFAULT IS INVERTED
//
// Earlier revisions only scanned sentences matching an enforcement vocabulary.
// Review appended three plain sentences using none of the 21 terms — the last,
// "Tool bodies always execute; the policy result is recorded alongside them",
// is the negation of the product — and every gate stayed green. Widening 3 -> 21
// closed the instance, not the class: a keyword allow-list cannot be completed,
// because whoever adds the claim picks the words after reading the list.
//
// The vocabulary now gates nothing. It survives as a severity hint in the
// failure message, and as the trigger for a stricter allow-list rule: an entry
// whose sentence reads like a claim needs a written justification, not a
// category.
//
// Section-level exclusions are gone too. An excluded section was a black hole —
// the guard checked the heading still existed and said nothing about its
// contents — so a claim inserted into "## Where to next" was never scanned.
//
// WHAT THIS GATE PROVES
//
//  1. Every sentence in the document is accounted for.
//  2. A binding matches a WHOLE sentence, exactly, and only one binding may.
//  3. Every control a binding names exists, extracted from the control files'
//     ASTs rather than transcribed.
//  4. Every claim is proven or openly unproven, and an unproven claim may not
//     name the ticket this file implements — that pointer resolves to a closed
//     issue the moment the work merges.
//  5. Comments are stripped before scanning, because a reader cannot see them.
//     CommonMark HTML blocks span blank lines, so the strip is non-greedy over
//     the whole document rather than per paragraph.
//
// WHAT THIS GATE DOES NOT PROVE
//
// It does not compile or execute a documented snippet. metadata/quickstart/
// holds .go.txt fragments precisely so the toolchain never sees them, and the
// docs-metadata generator check round-trips them as text. Binding a claim does
// not make it true.

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

const (
	quickStartDoc        = "../docs/quick-start.md"
	governanceErrorsFile = "governance_errors.go"

	// implementingTicket is the ticket this file delivers. An unproven claim may
	// not name it: on merge that pointer resolves to a closed issue, and the
	// ticket-shaped check is satisfied by any AAASM-nnnn, open or not.
	implementingTicket = "AAASM-5529"
)

// controlFiles are the test files a binding may name a control from.
var controlFiles = []string{
	"quickstart_negative_control_test.go",
	"audit_sink_test.go",
}

// enforcementVocabulary gates NOTHING. It is a severity hint in the failure
// message, and the trigger for the stricter allow-list rule. See the header for
// why gating on a keyword list is unsound.
var enforcementVocabulary = regexp.MustCompile(
	`(?i)\bdenie[sd]\b|\bdeny\b|\bblocked\b|\bblocking\b|\bnever runs?\b` +
		`|\bbefore execution\b|\bchecked against\b|\benforces?\b|\benforced\b` +
		`|\bpassthrough\b|\bdiscards?\b|\bdiscarded\b|\bthrows?\b|\brejects?\b` +
		`|\brouted\b|\bintercepts?\b|\binterception\b|\bgovern(s|ed|ance)?\b` +
		`|\bverified\b|\bprotection\b|\bunprotected\b|\bbypass(ed|es)?\b`,
)

// Allow-list categories. Permitted only for a sentence that does NOT match the
// vocabulary; anything that does needs a written reason.
const (
	notAClaim  = "Descriptive or instructional prose. Says nothing about what governance does to a tool call."
	navigation = "A cross-reference. The claim, if any, lives on the page linked to and is gated there."
)

// claimBinding is one documented claim and the controls that stand behind it.
type claimBinding struct {
	id string
	// quote is the claim as a WHOLE sentence, flattened. Compared with ==.
	quote string
	// controls are "<file> :: TestName/Subtest" ids drawn from controlFiles.
	controls []string
	// unprovenReason is set when no control proves the claim. Must name a
	// ticket, and must not name implementingTicket.
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
		// Ends with a colon: it introduces the fenced sample that follows.
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
		// Two claims in one sentence, so controls for both. The "discards" half
		// is bound to the audit-sink suite, which drives the SHIPPED client; it
		// was previously bound to a negative control whose fixture client
		// RETAINS the record, so that control stayed green when the claim
		// became false.
		controls: []string{
			negControl + "TestQuickStartFilesystemNegativeControl/PositiveControl_AllowedWriteCreatesTheFile",
			negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			auditControl + "TestShippedClientDeclaringNoRetentionReachesNothing",
			auditControl + "TestEveryShippedGovernanceClientDeclaresItsAuditSink",
		},
	},
	{
		id: "init-succeeds-once-a-gateway-is-reachable",
		// RESTORED. Revision 2 dropped this binding when the scan narrowed, so
		// coverage went DOWN between rounds and the only in-code pointer to
		// AAASM-5662 went with it — while the sentence stayed published and
		// stayed false. Inverting the default makes that class of regression
		// impossible: the sentence is scanned whether or not anyone remembers it.
		quote: "**`Init` succeeds** once a gateway is reachable (resolved or auto-started).",
		unprovenReason: "AAASM-5662: the documented \"Putting it together\" program was executed and " +
			"exits 1 — Init fails before any tool call, because WrapTools is reached with no " +
			"governance available. No control covers the documented program, and binding one of " +
			"the WrapTools controls here would misrepresent what was measured.",
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

// allowedSentences are every sentence that makes no capability claim, keyed
// exactly. A category is permitted only where the sentence does not match
// enforcementVocabulary; anything that does carries a written justification.
var allowedSentences = map[string]string{
	"The whole thing is a single `main` you can copy, paste, and run.": notAClaim,
	"The default pure-Go build has no native transport, so it does not register even when [`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress) is set (see that option's godoc).":                                          notAClaim,
	"Publishing the native library — or dropping the cgo requirement — is a separate product decision; track status in [AAASM-4547](https://lightning-dust-mite.atlassian.net/browse/AAASM-4547) and [AAASM-4469](https://lightning-dust-mite.atlassian.net/browse/AAASM-4469).": notAClaim,
	"{{< /callout >}}": notAClaim,
	"**Go** ≥ 1.26 (the floor declared in `go.mod`).": notAClaim,
	"For **local development**: nothing else — `Init` auto-discovers a gateway on `http://localhost:7391`, and starts one for you if none is running (the [`aasm` CLI](https://github.com/ai-agent-assembly/agent-assembly) must be on your `PATH`).": notAClaim,
	"{{< callout type=\"note\" >}} **Local-mode transports — `:7391` REST + `:50051` gRPC.** `Init` shells out to the following command to auto-start the gateway:":                                                                                   notAClaim,
	"The `:7391` auto-discovery above only resolves the REST gateway URL.": notAClaim,
	"Agent **registration** is a separate concern that talks to the gateway's gRPC endpoint (default `127.0.0.1:50051`) — `Init` does **not** auto-derive this address the way the Python and Node SDKs do.":                            notAClaim,
	"Reaching it requires an explicit [`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress) (or `WithSidecarBinary`) option; without one, `Init` returns `ErrSidecarUnavailable`.": notAClaim,
	"And per the warning at the top of this page, the registration handshake itself only runs under the opt-in native cgo binding today.":                                                                                               notAClaim,
	"To confirm both surfaces are actually up rather than guessing from `Init`'s behavior, check them directly:":                                                                                                                        notAClaim,
	"For **production**: a gateway URL and, if your gateway requires auth, an API key.":                                                                                                                                                 notAClaim,
	"Both can come from options, environment variables, or a config file — see [Configuration]({{< relref \"/configuration\" >}}).":                                                                                                     notAClaim,
	"*(Optional)* a C compiler, only if you opt into the native FFI transport with `-tags aa_ffi_go`.":                                                                                                                                  notAClaim,
	"The default transport is pure-Go and needs none.":                                            notAClaim,
	"`Init` returns an `*assembly.Assembly` — your runtime handle.":                               notAClaim,
	"Always `Close` it when you're done so the connection (and any managed sidecar) is released.": notAClaim,
	"For **local development** you can drop both options entirely — `assembly.Init(ctx)` resolves the gateway from the environment, then `~/.aasm/config.yaml`, then the local default.": notAClaim,
	"See [Configuration]({{< relref \"/configuration#gateway-and-credential-resolution\" >}}) for the full resolution order.":                                                            navigation,
	"Your tools just need to satisfy the SDK's small `Tool` interface:":                                                                                                                  notAClaim,
	"The second argument is the `GovernanceClient` that talks to the gateway.":                                                                                                           notAClaim,
	"Go's per-framework surface is thin today, so the tabs are **LangChainGo** (the framework path) and **Plain** (the framework-agnostic path).":                                        notAClaim,
	"A new tab appears automatically once a new Go **framework** example lands.":                                                                                                         notAClaim,
	"{{< tabs >}} {{< tab name=\"LangChainGo\" >}}":                                                                                                                                      notAClaim,
	"_Governance slice from the runnable `go/langchaingo/main.go` example._":                                                                                                             notAClaim,
	"{{< /tab >}} {{< tab name=\"Plain\" >}}":                                                                                                                                            notAClaim,
	"_Governance slice from the runnable `go/basic-agent/main.go` example._":                                                                                                             notAClaim,
	"{{< /tab >}} {{< /tabs >}}": notAClaim,
	"If no gateway can be found and no `aasm` binary is on `PATH`, you'll get a typed `*assembly.ConfigurationError` — see [Troubleshooting]({{< relref \"/troubleshooting\" >}}).": notAClaim,
	"**Tool calls run** and return the inner tool's result.":                                                     notAClaim,
	"[Core Concepts]({{< relref \"/core-concepts\" >}}) — what's actually happening inside the SDK.":             navigation,
	"**[Examples]({{< relref \"/examples\" >}})** — wire the SDK into the framework you actually use.":           navigation,
	"[Guides]({{< relref \"/guides\" >}}) — wrap a real agent, integrate a framework, handle decisions.":         navigation,
	"[Configuration]({{< relref \"/configuration\" >}}) — every `Init` option, defaults, and enforcement modes.": navigation,
	"[Troubleshooting]({{< relref \"/troubleshooting\" >}}) — what to do when `Init` or a check fails.":          navigation,

	// --- sentences matching the vocabulary: written justification required ---

	"{{< callout type=\"warning\" >}} **Agent registration is not reachable from a plain `go get` " +
		"today — following this quick-start will not make your agent appear in the dashboard.** The " +
		"steps below wrap and govern tool calls, but the register handshake runs *only* under the " +
		"opt-in native cgo binding (`-tags aa_ffi_go`, `CGO_ENABLED=1`), and that native library " +
		"(`libaa_ffi_go`) is **not published anywhere** yet: building with `-tags aa_ffi_go` fails " +
		"with `ld: library 'aa_ffi_go' not found` outside a full monorepo checkout.": "A LIMITATION " +
		"disclaimer, not a capability claim — it says what the product does NOT do, which is the " +
		"opposite of the over-claiming this gate exists to catch. Independently corroborated while " +
		"scoping AAASM-5529: native/aa-ffi-go/Cargo.toml sets publish = false and no release " +
		"workflow ships the library. Kept as an exact sentence so that if it ever becomes false, " +
		"editing it forces a revisit here. NOTE: no Jira ticket currently owns closing this gap.",

	"Hand `governed` to your agent in place of the originals.": "Names the sample's variable. It " +
		"matches the vocabulary only because the variable is called `governed`; it asserts nothing " +
		"about what happens to a call made through it.",

	"Pick your framework — each tab shows the governance slice copied verbatim from a runnable " +
		"example in the [examples repo](https://github.com/ai-agent-assembly/examples/tree/HEAD/go) " +
		"(a CI drift check keeps them in lockstep).": "Describes where the tab content comes from. " +
		"'governance slice' names the excerpt's provenance, not an enforcement outcome.",

	"Two more validated Go examples already exist — **Tool Policy** and **CLI Runtime (sidecar)** " +
		"— but those are patterns (an allow/deny policy demo and sidecar wiring), not \"first agent\" " +
		"frameworks, so they're intentionally left out of this quick-start; see " +
		"`metadata/quickstart/README.md` for the tab-selection rationale.": "An editorial note about " +
		"which example tabs the page shows. It matches the vocabulary through the phrase " +
		"'allow/deny policy demo', which names an example, not a guarantee.",
}

var (
	yamlFrontMatter = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)
	fencedCodeBlock = regexp.MustCompile("(?s)```.*?```")
	// A reader cannot see a comment, so a bound claim commented out of the
	// rendered page must not still satisfy this gate. CommonMark HTML blocks
	// (type 2) span blank lines, so this is applied to the whole document
	// before paragraph splitting, not per paragraph.
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	mdxComment  = regexp.MustCompile(`(?s)\{/\*.*?\*/\}`)
	listMarker  = regexp.MustCompile(`(?m)^\s*(?:[-*+]|\d+\.)\s+`)
	// Go's RE2 has no lookahead, so units are split by scanning lines rather
	// than with a zero-width pattern. A line that opens a list item or a table
	// row starts a new unit, so two adjacent bullets never merge into one
	// "sentence" that a single binding could then cover.
	unitOpener  = regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s|^\|`)
	headingLine = regexp.MustCompile(`(?m)^#{1,6} .*$`)
	// '.' and '?' only. '!' is not a terminator: Hugo/mkdocs callouts and
	// emphatic prose would otherwise split into fragments.
	sentenceEnd = regexp.MustCompile(`(?s)[.?]\s`)
	ticketRef   = regexp.MustCompile(`AAASM-\d+`)
)

func flattenMarkdown(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func readDocument(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(quickStartDoc)
	if err != nil {
		t.Fatalf("cannot read %s: %v", quickStartDoc, err)
	}
	// Normalise line endings first: on a CRLF checkout the paragraph split
	// never fires and the whole section collapses into one sentence.
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// splitUnits breaks a paragraph at each list item or table row.
func splitUnits(paragraph string) []string {
	var units []string
	var current []string
	for _, line := range strings.Split(paragraph, "\n") {
		if unitOpener.MatchString(line) && len(current) > 0 {
			units = append(units, strings.Join(current, "\n"))
			current = current[:0]
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		units = append(units, strings.Join(current, "\n"))
	}
	return units
}

func splitSentences(paragraph string) []string {
	locs := sentenceEnd.FindAllStringIndex(paragraph, -1)
	out := make([]string, 0, len(locs)+1)
	start := 0
	for _, loc := range locs {
		// Keep the terminator with the sentence it ends.
		out = append(out, paragraph[start:loc[0]+1])
		start = loc[1]
	}
	return append(out, paragraph[start:])
}

// scannedSentences returns flattened sentence -> section heading for the WHOLE
// document. No section is skipped.
func scannedSentences(t *testing.T) map[string]string {
	t.Helper()

	body := readDocument(t)
	body = yamlFrontMatter.ReplaceAllString(body, "")
	for _, pattern := range []*regexp.Regexp{fencedCodeBlock, htmlComment, mdxComment} {
		body = pattern.ReplaceAllString(body, "\n\n")
	}

	sentences := make(map[string]string)
	section := "(preamble)"
	offset := 0
	bounds := append(headingLine.FindAllStringIndex(body, -1), []int{len(body), len(body)})
	for _, loc := range bounds {
		for _, paragraph := range strings.Split(body[offset:loc[0]], "\n\n") {
			for _, unit := range splitUnits(paragraph) {
				for _, raw := range splitSentences(listMarker.ReplaceAllString(unit, "")) {
					if flat := flattenMarkdown(raw); flat != "" {
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

// sentinelNamesByMessage AST-parses governance_errors.go for
// `var Err… = errors.New("…")` pairs, so the sentinel's NAME is derived rather
// than transcribed. A rename tool renames the identifier but not a string
// literal, so a hard-coded name in a Contains check against the document passes
// while the SDK exports one name and the docs name another.
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

// TestClaimGateCanSeeWhatItGates is the positive control for the gate itself.
// An empty parse and a clean result look identical.
func TestClaimGateCanSeeWhatItGates(t *testing.T) {
	t.Run("TheWholeDocumentIsReadAndSplit", func(t *testing.T) {
		if n := len(scannedSentences(t)); n < 30 {
			t.Fatalf("only %d sentences parsed from the whole quick-start", n)
		}
	})

	t.Run("TheScanReachesEverySectionIncludingTheLast", func(t *testing.T) {
		sections := make(map[string]bool)
		for _, section := range scannedSentences(t) {
			sections[section] = true
		}
		if len(sections) < 5 {
			t.Fatalf("the scan reached only %d sections: %v", len(sections), sections)
		}
		// "## Where to next" used to be excluded by name, which made it a black
		// hole: a claim inserted there was never seen.
		if !sections["## Where to next"] {
			t.Fatalf("'## Where to next' is not in the scan; it must be. Sections seen: %v", sections)
		}
	})

	t.Run("CommentsAreStrippedBeforeScanning", func(t *testing.T) {
		document := readDocument(t)
		if !strings.Contains(document, "<!--") {
			t.Skip("the quick-start no longer contains an HTML comment; nothing to prove here")
		}
		for sentence := range scannedSentences(t) {
			if strings.Contains(sentence, "<!--") || strings.Contains(sentence, "-->") {
				t.Fatalf("HTML comment markup leaked into the scan: %q", sentence)
			}
		}
	})

	t.Run("TheASTExtractionFindsControlsInEveryControlFile", func(t *testing.T) {
		ids := controlNodeIDs(t)
		for _, file := range controlFiles {
			found := false
			for id := range ids {
				if strings.HasPrefix(id, file+" :: ") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("AST extraction found no controls in %s", file)
			}
		}
	})

	t.Run("TheSentinelExtractionFindsExportedErrorVars", func(t *testing.T) {
		if n := len(sentinelNamesByMessage(t)); n < 2 {
			t.Fatalf("AST extraction found %d sentinel vars in %s", n, governanceErrorsFile)
		}
	})
}

// TestEverySentenceIsAccountedFor is the inversion: bound, or allow-listed, or
// the gate fails. There is no third state and no keyword filter.
func TestEverySentenceIsAccountedFor(t *testing.T) {
	t.Run("NoSentenceIsUnboundAndUnallowed", func(t *testing.T) {
		quotes := make(map[string]bool, len(quickStartClaimBindings))
		for _, binding := range quickStartClaimBindings {
			quotes[binding.quote] = true
		}
		for sentence, section := range scannedSentences(t) {
			if quotes[sentence] {
				continue
			}
			if _, allowed := allowedSentences[sentence]; allowed {
				continue
			}
			severity := "prose"
			if enforcementVocabulary.MatchString(sentence) {
				severity = "CLAIM-LIKE"
			}
			t.Errorf("[%s] this sentence in section %q is neither bound nor allow-listed:\n  %q\n\n"+
				"CLAIM-LIKE marks a vocabulary match — a severity hint only. A sentence marked "+
				"'prose' can still be a claim, which is exactly why this gate does not filter on "+
				"the vocabulary.\n"+
				"Add a claimBinding whose quote is the WHOLE sentence and which names the control "+
				"that proves it (or an unprovenReason naming a ticket), or add it to "+
				"allowedSentences with a category. A vocabulary match needs a written "+
				"justification there, not a category.", severity, section, sentence)
		}
	})

	t.Run("NothingIsBothBoundAndAllowed", func(t *testing.T) {
		for _, binding := range quickStartClaimBindings {
			if _, allowed := allowedSentences[binding.quote]; allowed {
				t.Errorf("claim %q is both bound and allow-listed", binding.id)
			}
		}
	})

	t.Run("EachBindingMatchesExactlyOneWholeSentence", func(t *testing.T) {
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
					"The claim was reworded, split, merged, or commented out.",
					binding.id, quickStartDoc, matches, binding.quote)
			}
		}
	})

	t.Run("NoTwoBindingsClaimTheSameSentence", func(t *testing.T) {
		seen := make(map[string]string)
		for _, binding := range quickStartClaimBindings {
			if other, dup := seen[binding.quote]; dup {
				t.Errorf("claim bindings %q and %q quote the same sentence", other, binding.id)
			}
			seen[binding.quote] = binding.id
		}
	})
}

// TestTheAllowListCannotBecomeABypass keeps the exclusion list from becoming
// the new hole.
func TestTheAllowListCannotBecomeABypass(t *testing.T) {
	t.Run("EveryAllowedSentenceIsStillPresentVerbatim", func(t *testing.T) {
		scanned := scannedSentences(t)
		for sentence := range allowedSentences {
			if _, present := scanned[sentence]; !present {
				t.Errorf("allowedSentences contains a sentence that no longer appears in %s:\n  %q\n"+
					"It was reworded or removed. Delete the stale entry, and if the replacement makes "+
					"a capability claim, bind it.", quickStartDoc, sentence)
			}
		}
	})

	t.Run("AClaimLikeSentenceNeedsAWrittenJustification", func(t *testing.T) {
		// Waving through a sentence that reads like a claim must cost a
		// sentence of prose. The categories are deliberately unusable here.
		for sentence, reason := range allowedSentences {
			if !enforcementVocabulary.MatchString(sentence) {
				continue
			}
			if reason == notAClaim || reason == navigation {
				t.Errorf("this allow-listed sentence matches the enforcement vocabulary but is waved "+
					"through with a bare category:\n  %q\n"+
					"Replace the category with a written justification saying why it makes no "+
					"capability claim, or bind it.", sentence)
			}
		}
	})

	t.Run("EveryAllowListReasonIsNonEmpty", func(t *testing.T) {
		for sentence, reason := range allowedSentences {
			if strings.TrimSpace(reason) == "" {
				t.Errorf("allow-list entry has no reason: %q", sentence)
			}
		}
	})
}

// TestEveryBindingNamesSomethingReal fails when a control a binding depends on
// is renamed or removed, or when an unproven claim is untraceable.
func TestEveryBindingNamesSomethingReal(t *testing.T) {
	t.Run("NamedControlsExist", func(t *testing.T) {
		available := controlNodeIDs(t)
		for _, binding := range quickStartClaimBindings {
			for _, control := range binding.controls {
				if !available[control] {
					t.Errorf("claim binding %q names a control that does not exist:\n  %s",
						binding.id, control)
				}
			}
		}
	})

	t.Run("AClaimIsEitherProvenOrOpenlyUnproven", func(t *testing.T) {
		for _, binding := range quickStartClaimBindings {
			if len(binding.controls) > 0 {
				continue
			}
			if binding.unprovenReason == "" {
				t.Errorf("claim %q names no control and gives no unprovenReason", binding.id)
				continue
			}
			if !ticketRef.MatchString(binding.unprovenReason) {
				t.Errorf("claim %q is unproven but names no ticket. Reason: %q",
					binding.id, binding.unprovenReason)
			}
		}
	})

	t.Run("AnUnprovenReasonDoesNotNameTheImplementingTicket", func(t *testing.T) {
		// A reason naming this file's own ticket resolves to a CLOSED issue the
		// moment this work merges, and nothing would notice: the ticket-shaped
		// check above is satisfied by any AAASM-nnnn, open or not.
		for _, binding := range quickStartClaimBindings {
			if strings.Contains(binding.unprovenReason, implementingTicket) {
				t.Errorf("claim %q is registered unproven against %s, the ticket this file "+
					"implements. On merge that pointer resolves to a closed issue and the claim is "+
					"silently orphaned. Name the ticket that will actually resolve it, or file one.",
					binding.id, implementingTicket)
			}
		}
	})
}

// TestTheDocumentedErrorTypeIsTheOneTheSDKReturns drives a real deny through the
// exported WrapTools API and reads the concrete type name off the returned
// error with reflection. The type is deliberately NOT named here: naming it
// would make a rename a compile error, which aborts the package before this
// assertion can run.
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
		t.Fatalf("%s no longer names any *assembly.<Name>Error type", quickStartDoc)
	}
	for _, name := range documented {
		if name == actual {
			return
		}
	}
	t.Errorf("a denied call returns %s, but %s names only %v.\n"+
		"Either the type was renamed and the documentation now points at something a reader "+
		"cannot find, or the deny path changed which error it returns.",
		actual, filepath.Base(quickStartDoc), documented)
}

// TestTheDocumentedSentinelIsTheOneTheNilClientReturns pins the sentinel the
// prose tells a reader to match on, with the identifier DERIVED from source.
func TestTheDocumentedSentinelIsTheOneTheNilClientReturns(t *testing.T) {
	tool := newFileSideEffectTool(t)

	governed := WrapTools([]Tool{tool}, nil)
	_, err := governed[0].Call(WithAgentID(context.Background(), "claim-binding-agent"), "degraded")

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
		t.Fatalf("the nil-client deny returned %q, which matches no `var Err… = errors.New(…)` in %s. "+
			"Re-point this check rather than deleting it. Known sentinels: %v",
			err.Error(), governanceErrorsFile, byMessage)
	}

	if !strings.Contains(readDocument(t), derivedName) {
		t.Errorf("the nil-client deny path returns %s, but %s does not name it.\n"+
			"A reader is told the call is denied without being told what to match on — or the "+
			"identifier was renamed and the documentation still names the old one.",
			derivedName, filepath.Base(quickStartDoc))
	}
}
