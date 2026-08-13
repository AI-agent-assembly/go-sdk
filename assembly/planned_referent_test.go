// AAASM-5750 — AAASM-5731 must not be the referent of a forward-looking claim.
//
// The rule this gate enforces comes from **AAASM-5750's own description**, not
// from ADR 0033 §6. §6 requires that `Planned` carry *a* ticket reference and
// says nothing about which ticket; naming the right one is 5750's decision.
// Stating the source precisely matters, because a failure message that cites an
// ADR for a rule the ADR does not contain sends the next reader to the wrong
// document.
//
// The defect: AAASM-5731 measured that this SDK's shipped client drops the
// hook-layer audit record. It never intended to build a sink, and it is closed.
// A forward-looking claim pointing at it reads as a live commitment while
// resolving to finished work. So the invariant is narrow and permanent —
// **AAASM-5731 may be cited as the ticket that measured the drop, never as the
// ticket that will fix it.**
//
// The assertion is two-tier, because one tier alone fails in one direction or
// the other and review caught both:
//
//   - a **guarded** site (one of [expectedSites]) must name AAASM-5750 exactly.
//     Without this the gate stops asserting the thing the change made true —
//     repointing a guarded site to any other live ticket passed green, which is
//     precisely the drift the gate exists to catch.
//   - **any other** site must merely not name a stale referent. Asserting
//     AAASM-5750 repo-wide was the first version's defect: §6 scopes `Planned`
//     to any decided-but-unbuilt capability with any ticket, so an unrelated
//     roadmap row is legitimate and must not fail this gate.
//
// Two limits are disclosed rather than fixed, both measured as currently
// unreachable:
//
//   - the reachability check is per **file**, not per site. A guarded file that
//     reflowed its real site out of the scan's reach AND gained a second,
//     correct claim would keep its entry. Requires two coordinated edits; today
//     no file in this module carries more than one site.
//   - the gate file is excluded from its own scan, so it is a hiding place for
//     a stale referent. It is a test file that documents no SDK behaviour, and
//     the exclusion matches one exact path rather than a prefix.
package assembly

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// capabilityReferent is the ticket that owns building the SDK-side audit sink.
// Guarded sites must name it exactly.
const capabilityReferent = "AAASM-5750"

// staleReferents are the tickets that *measured* the absence of an SDK-side
// audit sink. Backward citations to them are correct and are left alone; what
// this gate forbids is either one appearing as the ticket a forward-looking
// claim defers to.
var staleReferents = map[string]bool{
	"AAASM-5731": true,
	"AAASM-5681": true,
}

// forwardClaim matches the two shapes a deferral takes here: the ADR 0033 §6
// term, and the plain "tracked as" pointer used where no term is stated.
var (
	forwardClaim = regexp.MustCompile(`\bPlanned\b|tracked as`)
	ticketRefRe  = regexp.MustCompile(`AAASM-\d+`)
	scannedFiles = regexp.MustCompile(`\.(go|md)$`)
)

// gateFile is excluded from its own scan. Including it was the first version's
// defect: this file names AAASM-5750 in its own header, which padded the site
// count and let a floor be satisfied entirely by the gate quoting itself.
const gateFile = "assembly/planned_referent_test.go"

// expectedSites are the audit-sink deferrals that must remain reachable by the
// scan. This is a fixture compared against a walk of the tree, not a constant
// compared against another constant: if a site is deleted, renamed, or reflowed
// out of the scan's reach, the walk stops finding it and this fails.
//
// Reflow is the realistic case and the one the first version missed — a comment
// rewrapped so the term and the ticket land on different lines silently left
// the scan.
var expectedSites = []string{
	"assembly/audit_sink.go",
	"assembly/ffi_governance_client.go",
	"assembly/tool_wrapper.go",
	"assembly/op_control_gate_test.go",
	"assembly/quickstart_negative_control_test.go",
}

// moduleRoot walks up from the test's working directory to the directory
// holding go.mod. Resolving it rather than assuming a relative path keeps the
// scan module-wide regardless of where `go test` is invoked from.
func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory; the scan would " +
				"cover nothing and pass for the wrong reason")
		}

		dir = parent
	}
}

// endsSentence reports whether a comment line closes a sentence. A wrapped
// sentence (the Python SDK's `… Planned under ADR 0033 §6` / `(AAASM-5750) …`)
// does not; a complete one does.
func endsSentence(line string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(line), "*/ ")
	if trimmed == "" {
		return false
	}

	switch trimmed[len(trimmed)-1] {
	case '.', '!', '?':
		return true
	}

	return false
}

type deferralSite struct {
	path   string
	line   int
	ticket string
	text   string
}

// findDeferralSites returns every forward-looking claim paired with a ticket.
//
// The ticket is looked for on the claim's own line **and the line after it**.
// One line is not enough: `test_quickstart_negative_control.py`'s equivalent in
// the Python SDK wraps `Planned` and its ticket onto separate lines, and a
// same-line-only scan silently skipped it. Whether a site is covered should not
// depend on where a comment happens to wrap.
func findDeferralSites(t *testing.T, root string) []deferralSite {
	t.Helper()

	var sites []deferralSite

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}

			return nil
		}

		if !scannedFiles.MatchString(path) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if rel == gateFile {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(body), "\n")
		for i, line := range lines {
			if !forwardClaim.MatchString(line) {
				continue
			}

			// Extend to the next line only when this line carries no ticket of
			// its own AND does not end a sentence. Without the sentence guard
			// the window pairs a claim with a ticket belonging to the *next*
			// sentence — review produced a real case where an inserted line of
			// forward-looking prose was blamed for a correct backward citation
			// beneath it. There are 30 such backward citations in this module.
			window := line
			if !ticketRefRe.MatchString(line) && !endsSentence(line) && i+1 < len(lines) {
				window += "\n" + lines[i+1]
			}

			ticket := ticketRefRe.FindString(window)
			if ticket == "" {
				continue
			}

			sites = append(sites, deferralSite{
				path:   rel,
				line:   i + 1,
				ticket: ticket,
				text:   strings.TrimSpace(line),
			})
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return sites
}

func TestForwardClaimsNameTheRightTicket(t *testing.T) {
	t.Parallel()

	guarded := make(map[string]bool, len(expectedSites))
	for _, path := range expectedSites {
		guarded[path] = true
	}

	for _, site := range findDeferralSites(t, moduleRoot(t)) {
		switch {
		case guarded[site.path]:
			if site.ticket != capabilityReferent {
				t.Errorf("%s:%d is a guarded audit-sink deferral and must name %s, "+
					"not %s — this is the site the referent change corrected, and "+
					"letting it drift to any other ticket is what this gate exists "+
					"to prevent: %s",
					site.path, site.line, capabilityReferent, site.ticket, site.text)
			}
		case staleReferents[site.ticket]:
			t.Errorf("%s:%d defers to %s, which measured the drop and will not fix "+
				"it — use the ticket that builds the sink (AAASM-5750, per its own "+
				"description): %s",
				site.path, site.line, site.ticket, site.text)
		}
	}
}

func TestEveryExpectedSiteIsStillReachable(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for _, site := range findDeferralSites(t, moduleRoot(t)) {
		seen[site.path] = true
	}

	// Anti-vacuity, and the reason it is a set of paths rather than a count: a
	// count can be held up by an unrelated site appearing as a real one is
	// deleted. Naming them makes that substitution visible.
	for _, want := range expectedSites {
		if !seen[want] {
			t.Errorf("%s carries no forward claim the scan can pair with a ticket; "+
				"it was deleted, renamed, or reflowed so the term and the ticket "+
				"are more than one line apart — in which case a stale referent "+
				"there would no longer be checked", want)
		}
	}
}
