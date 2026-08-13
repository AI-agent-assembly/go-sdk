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
// Deliberately NOT asserted: that every `Planned` in this repository names
// AAASM-5750. §6 scopes `Planned` to any decided-but-unbuilt capability with
// any ticket, so an unrelated roadmap row is legitimate and must not fail this
// gate. The first version of this test made exactly that over-broad assertion.
package assembly

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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

			window := line
			if i+1 < len(lines) {
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

func TestNoForwardClaimDefersToAClosedMeasurementTicket(t *testing.T) {
	t.Parallel()

	for _, site := range findDeferralSites(t, moduleRoot(t)) {
		if staleReferents[site.ticket] {
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
