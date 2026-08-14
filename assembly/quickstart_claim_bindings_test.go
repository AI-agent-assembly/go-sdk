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
//
// documented_programs_test.go joined the list for AAASM-5662. It is the only
// control here that EXECUTES what the page publishes, which is the gap this
// file's own header admits ("It does not compile or execute a documented
// snippet"): the "Putting it together" program was bound, allow-listed, and
// broken at the same time, and every check in this file stayed green.
var controlFiles = []string{
	"quickstart_negative_control_test.go",
	"audit_sink_test.go",
	"documented_programs_test.go",
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

// structural is the ONLY bare constant. It is permitted solely for lines that
// are not prose — a bare Hugo shortcode, a tab caption, a table row, a bare
// link-list item — matched by structuralLine below. Every other entry carries a
// written justification unique to that sentence.
//
// The previous rule required a justification only when the sentence matched the
// enforcement vocabulary, which is backwards: the sentences that most need
// explaining are the ones that EVADE the vocabulary, since evading it is the
// whole reason the scan was inverted.
const structural = "Structurally non-prose: a line that is entirely Hugo shortcodes and carries no sentence."

// structuralLine matches the lines that may use the bare constant.
//
// Fully anchored, and deliberately narrow. The previous pattern asked whether a
// sentence STARTED with structure, not whether it was ONLY structure, so a link
// item or a table row was waved through on its first characters while its
// anchor text — rendered prose a reader sees — went unexamined. A payload as
// plain as "[Every tool request is permitted to proceed…](x.md)" passed.
// Only a line that is entirely Hugo shortcodes qualifies now.
var structuralLine = regexp.MustCompile(`^\{\{<[^>]*>\}\}(\s*\{\{<[^>]*>\}\})*$`)

// negation / soConnector implement the "so" rule, which is deliberately not
// part of contrastiveConjunction above.
//
// The risk "so" carries is not adversativeness but POLARITY CHANGE. "We don't
// do X but Y" is a concession; "we don't do X so Y covers it" is a REASSURANCE,
// and reassurance is the register documentation over-claims in. A negated
// clause followed by an un-negated one is the shape that hides an affirmative
// capability claim behind a limitation.
//
// Flagging "so" flat would catch six live sentences across the three repos,
// every one of which turns the way it started — four of them in this file.
// This form catches none of them and still catches the payload, which is the
// only negative-to-positive case.
var (
	negation    = regexp.MustCompile(`(?i)\b(?:not|no|never|cannot|can't|without)\b`)
	soConnector = regexp.MustCompile(`(?i)\sso\s`)
)

// minJustification is a floor, not a real check: no gate can tell a
// justification from noise. It only makes reason="x" visible.
const minJustification = 40

// contrastiveConjunction marks a sentence that turns mid-way. Such a sentence
// can under-claim and over-claim at once, and a justification arguing "this only
// says what the product does NOT do" cannot be trusted for one that turns.
//
// Applied to EVERY allow-listed sentence, not only those whose justification
// calls itself a disclaimer. The previous version keyed off a marker phrase
// matched case-sensitively, which made the rule opt-in by the author it
// constrains: capitalising the phrase, or omitting it, evaded the check —
// and "A LIMITATION disclaimer…" is the wording this file shipped one round ago.
//
// "so" is deliberately excluded: it is consequential ("therefore") rather than
// adversative. Both attack payloads are still caught — the reviewer's used
// "because" and the live case here used "but".
var contrastiveConjunction = regexp.MustCompile(`(?i)\s(?:but|because|though|although|however|whereas|while)\s`)

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
	negControl     = "quickstart_negative_control_test.go :: "
	auditControl   = "audit_sink_test.go :: "
	programControl = "documented_programs_test.go :: "
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
		id: "init-reports-the-runtime-unavailable-on-the-default-build",
		// REPLACES "**`Init` succeeds** once a gateway is reachable (resolved or
		// auto-started)", which was registered unproven against AAASM-5662 and was
		// false in both halves. Init does not succeed on the default build at all:
		// the pure-Go binding's connect reports the runtime unavailable, so boot
		// falls through to connectToLocalSidecar, which returns an error
		// unconditionally. Reachability of the REST gateway is not what decides it.
		quote: "**`Init` reports the runtime unavailable** on the default pure-Go build: it returns " +
			"`ErrSidecarUnavailable` with or without " +
			"[`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress), " +
			"since that build links no native transport.",
		controls: []string{
			programControl + "TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns/InitReportsTheRuntimeUnavailableWithNoSidecarOption",
			programControl + "TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns/InitReportsTheRuntimeUnavailableWithASidecarAddressToo",
		},
	},
	{
		id: "the-sidecar-option-does-not-change-the-init-outcome",
		// The prerequisites callout said "without one, `Init` returns
		// ErrSidecarUnavailable", which reads as a promise about WITH one. The
		// control that falsifies it passes the option and measures the same error.
		quote: "On the default pure-Go build `Init` returns `ErrSidecarUnavailable` with or without " +
			"that option, because the build links no native transport and the fallback connector " +
			"reaches no sidecar.",
		controls: []string{
			programControl + "TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns/InitReportsTheRuntimeUnavailableWithASidecarAddressToo",
			programControl + "TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns/InitReportsTheRuntimeUnavailableWithNoSidecarOption",
		},
	},
	{
		id: "the-step-two-program-runs-and-exits-zero",
		// Bound to the executing gate, not to prose. The two halves falsify each
		// other's failure mode: delete the ErrSidecarUnavailable branch from the
		// published program and log.Fatalf fires, so the exit-0 control goes red.
		quote: "Run against a default `go get` install, this program reports `ErrSidecarUnavailable` " +
			"and exits 0.",
		controls: []string{
			programControl + "TestEveryDocumentedProgramRunsToCompletion",
			programControl + "TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns/InitReportsTheRuntimeUnavailableWithNoSidecarOption",
		},
	},
	{
		id: "steps-below-wrap-and-govern-tool-calls",
		// Review found this shipped as a DISCLAIMER-justified allow-list entry.
		// It is not a disclaimer: the clause before the turn — "The steps below
		// wrap and govern tool calls" — is an affirmative capability claim, and
		// it rode along under a justification reading "it says what the product
		// does NOT do". True today and provable, which is exactly why the
		// precedent was the danger rather than the sentence.
		quote: "The steps below wrap and govern tool calls, but the register handshake runs *only* under " +
			"the opt-in native cgo binding (`-tags aa_ffi_go`, `CGO_ENABLED=1`), and that native library " +
			"(`libaa_ffi_go`) is **not published anywhere** yet: building with `-tags aa_ffi_go` fails " +
			"with `ld: library 'aa_ffi_go' not found` outside a full monorepo checkout.",
		controls: []string{
			negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			negControl + "TestQuickStartNetworkNegativeControl/NegativeControl_DeniedEgressNeverReachesTheListener",
			negControl + "TestQuickStartWrappingDoesNotDisarmTheOriginalTool",
		},
	},
	{
		id: "the-governed-call-returns-the-inner-result",
		// The half the old page got wrong. It read "**Tool calls run** and return
		// the inner tool's result" directly under a program that passed nil, where
		// the call is denied — the same page asserting both. It is true of the
		// wiring the program now shows, and the output control is what ties it to
		// that program rather than to a claim about tool calls in general.
		quote: "**The governed call returns the inner tool's result**, because `localPolicy` " +
			"evaluated it and allowed it.",
		controls: []string{
			programControl + "TestDocumentedOutputCommentsMatchTheRun",
			negControl + "TestQuickStartFilesystemNegativeControl/PositiveControl_AllowedWriteCreatesTheFile",
		},
	},
	{
		id: "deny-surfaces-as-policy-violation-and-tool-body-does-not-run",
		quote: "**A `Decision{Denied: true}` surfaces as a `*assembly.PolicyViolationError`** and the " +
			"tool body is Denied before execution.",
		controls: []string{
			negControl + "TestQuickStartFilesystemNegativeControl/NegativeControl_DeniedWriteLeavesNoFile",
			negControl + "TestQuickStartNetworkNegativeControl/NegativeControl_DeniedEgressNeverReachesTheListener",
			negControl + "TestQuickStartWrappingDoesNotDisarmTheOriginalTool",
		},
	},
	{
		id: "substituting-nil-denies-before-execution",
		quote: "**Substituting `nil` for `localPolicy`** leaves the call Denied before execution with " +
			"`ErrGovernanceUnavailable`, under the default fail-closed enforce posture.",
		controls: []string{
			negControl + "TestQuickStartDegradedPathCannotLookProtected/NilClientUnderTheDefaultPostureDeniesRatherThanRunning",
			negControl + "TestQuickStartDegradedPathCannotLookProtected/NilClientWithFailOpenRunsTheToolAndSaysSo",
		},
	},
}

// allowedSentences are every sentence that makes no capability claim, keyed
// exactly. A category is permitted only where the sentence does not match
// enforcementVocabulary; anything that does carries a written justification.
var allowedSentences = map[string]string{
	"Two more validated Go examples already exist — **Tool Policy** and **CLI Runtime (sidecar)**.":                                                                                                                                                                              "Names two examples that exist outside this quick-start. An inventory statement about the examples repo.",
	"Those are patterns (an allow/deny policy demo and sidecar wiring), not \"first agent\" frameworks.":                                                                                                                                                                         "Classifies the two examples that are not shown as tabs. An inventory statement about what they are.",
	"They're intentionally left out of this quick-start; see `metadata/quickstart/README.md` for the tab-selection rationale.":                                                                                                                                                   "Explains the tab-selection decision and points at its rationale. Editorial, about the page rather than the product.",
	"*(Optional)* a C compiler, only if you opt into the native FFI transport with `-tags aa_ffi_go`.":                                                                                                                                                                           "A prerequisites bullet naming a build dependency and when it is needed.",
	"**Go** ≥ 1.26 (the floor declared in `go.mod`).":                                                                                                                                                                                                                            "A prerequisites bullet naming the language version floor.",
	"**[Examples]({{< relref \"/examples\" >}})** — wire the SDK into the framework you actually use.":                                                                                                                                                                           "A Where-to-next link item pointing at the examples index. An invitation to read further.",
	"A new tab appears automatically once a new Go **framework** example lands.":                                                                                                                                                                                                 "Describes how the tab list is generated. Documentation mechanics.",
	"Agent **registration** is a separate concern that talks to the gateway's gRPC endpoint (default `127.0.0.1:50051`) — `Init` does **not** auto-derive this address the way the Python and Node SDKs do.":                                                                     "Distinguishes registration from gateway resolution and states that Go does less here than the other SDKs. A narrowing statement.",
	"Always `Close` it when you're done so the connection (and any managed sidecar) is released.":                                                                                                                                                                                "Lifecycle instruction for the handle returned by Init.",
	"And per the warning at the top of this page, the registration handshake itself only runs under the opt-in native cgo binding today.":                                                                                                                                        "Repeats the registration limitation from the page's warning callout. A restriction, and it names no capability the SDK is claimed to have.",
	"Both can come from options, environment variables, or a config file — see [Configuration]({{< relref \"/configuration\" >}}).":                                                                                                                                              "Lists where the gateway URL and API key may be configured from.",
	"For **local development** you can drop both options entirely — `assembly.Init(ctx)` resolves the gateway from the environment, then `~/.aasm/config.yaml`, then the local default.":                                                                                         "Lists the gateway resolution order for local development.",
	"For **local development**: nothing else — `Init` auto-discovers a gateway on `http://localhost:7391`, and starts one for you if none is running (the [`aasm` CLI](https://github.com/ai-agent-assembly/agent-assembly) must be on your `PATH`).":                            "A prerequisites bullet describing auto-discovery and its PATH precondition.",
	"For **production**: a gateway URL and, if your gateway requires auth, an API key.":                                                                                                                                                                                          "Names what a production deployment must supply.",
	"Go's per-framework surface is thin today, so the tabs are **LangChainGo** (the framework path) and **Plain** (the framework-agnostic path).":                                                                                                                                "Explains why only two tabs exist. Editorial, about the page rather than the product.",
	"Hand `governed` to your agent in place of the originals.":                                                                                                                                                                                                                   "Names the sample's variable. It reads like a claim only because that variable is called `governed`.",
	"Pick your framework — each tab shows the governance slice copied verbatim from a runnable example in the [examples repo](https://github.com/ai-agent-assembly/examples/tree/HEAD/go) (a CI drift check keeps them in lockstep).":                                            "Describes where the tab content comes from. 'governance slice' names the excerpt's provenance, not an outcome.",
	"Publishing the native library — or dropping the cgo requirement — is a separate product decision; track status in [AAASM-4547](https://lightning-dust-mite.atlassian.net/browse/AAASM-4547) and [AAASM-4469](https://lightning-dust-mite.atlassian.net/browse/AAASM-4469).": "Points at the tickets tracking the native-library decision. Roadmap pointer, claiming nothing about today.",
	"Reaching it requires an explicit [`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress) (or `WithSidecarBinary`) option.":                                                                                               "Names the option the gRPC registration endpoint requires. A precondition on its own; the sentence that follows states the outcome and is bound.",
	"See [Troubleshooting]({{< relref \"/troubleshooting\" >}}).":                                                                                                                                                                                                                "A bare cross-reference closing the Init bullet, pointing at the page that covers recovery from a failed Init.",
	"See [Configuration]({{< relref \"/configuration#gateway-and-credential-resolution\" >}}) for the full resolution order.":                                                                                                                                                    "A cross-reference to the credential-resolution documentation.",
	"The `:7391` auto-discovery above only resolves the REST gateway URL.":                                                                                                                                                                                                       "Scopes what auto-discovery covers, narrowing rather than widening the claim above it.",
	"The default pure-Go build has no native transport, so it does not register even when [`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress) is set (see that option's godoc).":                                          "States that the default build cannot register at all. Purely a limitation, and it contains no affirmative capability clause.",
	"The default transport is pure-Go and needs none.":                                                                                                                                                                                                                           "States that the default build needs no C compiler, continuing the prerequisites bullet.",
	"The second argument is the `GovernanceClient` that talks to the gateway.":                                                                                                                                                                                                   "Names what the second WrapTools parameter is. A signature description.",
	"The whole thing is a single `main` you can copy, paste, and run.":                                                                                                                                                                                                           "Describes the shape of the example below it.",
	"To confirm both surfaces are actually up rather than guessing from `Init`'s behavior, check them directly:":                                                                                                                                                                 "Tells the reader to verify the ports themselves; the commands follow in a fenced block.",
	"Your tools just need to satisfy the SDK's small `Tool` interface:":                                                                                                                                                                                                          "Introduces the Tool interface listing that follows.",
	"[Configuration]({{< relref \"/configuration\" >}}) — every `Init` option, defaults, and enforcement modes.":                                                                                                                                                                 "A Where-to-next link item pointing at Configuration; the option reference lives on that page.",
	"[Core Concepts]({{< relref \"/core-concepts\" >}}) — what's actually happening inside the SDK.":                                                                                                                                                                             "A Where-to-next link item pointing at Core Concepts. Its claims live on that page and are gated there.",
	"[Guides]({{< relref \"/guides\" >}}) — wrap a real agent, integrate a framework, handle decisions.":                                                                                                                                                                         "A Where-to-next link item pointing at the Guides index, listing topics covered elsewhere.",
	"[Troubleshooting]({{< relref \"/troubleshooting\" >}}) — what to do when `Init` or a check fails.":                                                                                                                                                                          "A Where-to-next link item pointing at Troubleshooting, about recovery steps rather than what governance does.",
	"_Governance slice from the runnable `go/basic-agent/main.go` example._":                                                                                                                                                                                                     "An italic caption naming which example file the basic-agent tab was excerpted from.",
	"_Governance slice from the runnable `go/langchaingo/main.go` example._":                                                                                                                                                                                                     "An italic caption naming which example file the LangChainGo tab was excerpted from.",
	"`Init` returns an `*assembly.Assembly` — your runtime handle.":                                                                                                                                                                                                              "Names the concrete type Init returns. A signature description.",
	"`Init` shells out to the following command to auto-start the gateway:":                                                                                                                                                                                                      "Introduces the auto-start command shown in the fenced block below.",
	"{{< /callout >}}":                        structural,
	"{{< /tab >}} {{< /tabs >}}":              structural,
	"{{< /tab >}} {{< tab name=\"Plain\" >}}": structural,
	"{{< callout type=\"note\" >}} **Local-mode transports — `:7391` REST + `:50051` gRPC.**":                                                                                             "The bold heading of the transports callout; the surfaces it introduces are described in the sentences that follow.",
	"{{< callout type=\"warning\" >}} **Agent registration is not reachable from a plain `go get` today — following this quick-start will not make your agent appear in the dashboard.**": "A pure limitation disclaimer: it states only what the product does NOT do, with no affirmative clause attached. Corroborated while scoping AAASM-5529 — native/aa-ffi-go/Cargo.toml sets publish = false. AAASM-5767 owns closing the gap.",
	"{{< tabs >}} {{< tab name=\"LangChainGo\" >}}": structural,
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
	//
	// Closing markup between the terminator and the space is consumed WITH the
	// sentence, so a bold lead-in like "**… dashboard.**" ends there instead of
	// running into the sentence after it. Without that, an affirmative capability
	// clause and a disclaimer sat in one string and a single justification
	// covered both. A backtick is deliberately NOT in the trailing class: inline
	// code such as `phi.*` would otherwise be read as a sentence end.
	sentenceEnd = regexp.MustCompile("[.?][*)\\]_\"']*[ \t\n]")
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
		// Keep the terminator AND its closing markup with the sentence they end;
		// only the trailing whitespace byte is dropped.
		out = append(out, paragraph[start:loc[1]-1])
		start = loc[1] - 1
	}
	return append(out, paragraph[start:])
}

// occurrence is one sentence at one place in the document.
type occurrence struct {
	text    string
	section string
}

// scannedOccurrences returns EVERY (sentence, section) occurrence in the whole
// document. No section is skipped.
//
// A slice, not a map. Keying by sentence collapsed duplicates before anything
// counted them, so "matched == 1" could only ever be 0 or 1 and a bound true
// sentence could be pasted into a section that inverts its meaning — an
// observe-mode block, a "what not to do" block — and still count once. Section
// attribution was last-write-wins for the same reason.
func scannedOccurrences(t *testing.T) []occurrence {
	t.Helper()

	body := readDocument(t)
	body = yamlFrontMatter.ReplaceAllString(body, "")
	for _, pattern := range []*regexp.Regexp{fencedCodeBlock, htmlComment, mdxComment} {
		body = pattern.ReplaceAllString(body, "\n\n")
	}

	var occurrences []occurrence
	section := "(preamble)"
	offset := 0
	bounds := append(headingLine.FindAllStringIndex(body, -1), []int{len(body), len(body)})
	for _, loc := range bounds {
		for _, paragraph := range strings.Split(body[offset:loc[0]], "\n\n") {
			for _, unit := range splitUnits(paragraph) {
				for _, raw := range splitSentences(listMarker.ReplaceAllString(unit, "")) {
					if flat := flattenMarkdown(raw); flat != "" {
						occurrences = append(occurrences, occurrence{text: flat, section: section})
					}
				}
			}
		}
		if loc[1] > loc[0] {
			section = strings.TrimSpace(body[loc[0]:loc[1]])
		}
		offset = loc[1]
	}
	return occurrences
}

// scannedTexts is the de-duplicated set, for membership questions where the
// number of occurrences does not matter.
func scannedTexts(t *testing.T) map[string]bool {
	t.Helper()
	texts := make(map[string]bool)
	for _, occ := range scannedOccurrences(t) {
		texts[occ.text] = true
	}
	return texts
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
		if n := len(scannedOccurrences(t)); n < 30 {
			t.Fatalf("only %d sentences parsed from the whole quick-start", n)
		}
	})

	t.Run("TheScanReachesEverySectionIncludingTheLast", func(t *testing.T) {
		sections := make(map[string]bool)
		for _, occ := range scannedOccurrences(t) {
			sections[occ.section] = true
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
		for text := range scannedTexts(t) {
			if strings.Contains(text, "<!--") || strings.Contains(text, "-->") {
				t.Fatalf("HTML comment markup leaked into the scan: %q", text)
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
		reported := map[string]bool{}
		for _, occ := range scannedOccurrences(t) {
			sentence, section := occ.text, occ.section
			if quotes[sentence] || reported[sentence] {
				continue
			}
			if _, allowed := allowedSentences[sentence]; allowed {
				continue
			}
			reported[sentence] = true
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
		occurrences := scannedOccurrences(t)
		for _, binding := range quickStartClaimBindings {
			var where []string
			for _, occ := range occurrences {
				if occ.text == binding.quote {
					where = append(where, occ.section)
				}
			}
			if len(where) != 1 {
				t.Errorf("claim binding %q must match exactly one whole sentence occurrence in %s; "+
					"it matched %d (sections: %v).\nIts quote is:\n  %q\n"+
					"0 means the claim was reworded, split, merged, or commented out.\n"+
					"More than 1 means the same sentence now appears in more than one place, which is "+
					"not harmless: a true claim pasted into a section that inverts its meaning would "+
					"otherwise still be counted by this binding as the sentence that was proven.",
					binding.id, quickStartDoc, len(where), where, binding.quote)
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
		scanned := scannedTexts(t)
		for sentence := range allowedSentences {
			if !scanned[sentence] {
				t.Errorf("allowedSentences contains a sentence that no longer appears in %s:\n  %q\n"+
					"It was reworded or removed. Delete the stale entry, and if the replacement makes "+
					"a capability claim, bind it.", quickStartDoc, sentence)
			}
		}
	})

	t.Run("OnlyStructuralLinesMayUseTheBareConstant", func(t *testing.T) {
		// Every prose entry needs a written justification, not only the ones the
		// vocabulary already catches. Keying off the vocabulary was backwards: a
		// sentence that MATCHES it has already warned the author; one that
		// EVADES it has not, and evading it is why this scan was inverted.
		for sentence, reason := range allowedSentences {
			if reason == structural && !structuralLine.MatchString(sentence) {
				t.Errorf("this allow-listed sentence is prose but is waved through with the bare "+
					"structural constant:\n  %q\n"+
					"Replace it with a written justification saying why this particular sentence "+
					"makes no capability claim, or bind it.", sentence)
			}
		}
	})

	t.Run("NoAllowListedSentenceTurnsMidWay", func(t *testing.T) {
		// Rather than judge each case, reject the shape: an allow-listed
		// sentence may not contain a contrastive conjunction. It costs nothing
		// on genuine non-claims, because a sentence that turns should be split
		// regardless of what its justification says.
		for sentence := range allowedSentences {
			if contrastiveConjunction.MatchString(sentence) {
				t.Errorf("this allow-listed sentence contains a contrastive conjunction, so part of "+
					"it may be an affirmative capability claim riding along under its "+
					"justification:\n  %q\n"+
					"Split the affirmative clause into its own sentence and bind it to the controls "+
					"that prove it.", sentence)
			}
		}
	})

	t.Run("NoAllowListedSentenceReassuresAcrossANegation", func(t *testing.T) {
		// "Network-layer interception is not enabled by default, so the
		// in-process adapter verifies every outbound request before it leaves
		// the host instead." reads as a limitation and asserts a capability the
		// product does not have. contrastiveConjunction does not catch it,
		// because "so" is not adversative — it is the reassurance that follows
		// a denial, which is precisely where an unbacked claim hides.
		for sentence := range allowedSentences {
			for _, loc := range soConnector.FindAllStringIndex(sentence, -1) {
				before, after := sentence[:loc[0]], sentence[loc[1]:]
				if negation.MatchString(before) && !negation.MatchString(after) {
					t.Errorf("this allow-listed sentence negates something and then says \"so …\" "+
						"without a second negation — the shape of a limitation followed by a "+
						"reassurance, where the reassurance may be an unbacked capability "+
						"claim:\n  %q\n"+
						"Split the clause after \"so\" into its own sentence and bind it to the "+
						"controls that prove it.", sentence)
					break
				}
			}
		}
	})

	t.Run("WrittenJustificationsAreSubstantialAndDistinct", func(t *testing.T) {
		// A cheap partial, and only that. No gate can tell a justification from
		// noise — reason="x" is prose to a computer. Length and uniqueness only
		// make the two cheapest ways of waving something through, an empty
		// gesture and a copy-paste, visible in review.
		seen := map[string]string{}
		for sentence, reason := range allowedSentences {
			if reason == structural {
				continue
			}
			if len(reason) < minJustification {
				t.Errorf("justification shorter than %d characters for:\n  %q\n  reason: %q",
					minJustification, sentence, reason)
			}
			if other, dup := seen[reason]; dup {
				t.Errorf("the same justification is reused for two sentences:\n  %q\n  used by %q\n  and by %q\n"+
					"A justification explains one specific sentence; reuse is copy-paste waving.",
					reason, other, sentence)
			}
			seen[reason] = sentence
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
