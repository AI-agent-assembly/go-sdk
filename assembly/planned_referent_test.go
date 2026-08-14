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
// **AAASM-5750 built the sink, so it joined the list it used to be the answer
// to.** The gate previously required a fixed set of guarded files to name
// AAASM-5750 as the ticket their `Planned` deferred to. Once the capability
// exists there is nothing left to defer: a site still saying recording is
// *Planned* under AAASM-5750 is now describing a shipped behaviour as unbuilt,
// which is the same stale-pointer defect one ticket later. So the rule collapsed
// to a single tier — **no forward-looking claim in this module may defer
// SDK-side audit recording to any of the three tickets that are done with it** —
// and applies module-wide rather than to a named set.
//
// §6 still scopes `Planned` to any decided-but-unbuilt capability with any
// ticket, so an unrelated roadmap deferral naming some other ticket is
// legitimate and must not fail this gate. Only the three named referents are
// forbidden.
//
// A rule whose expected result is "no findings" needs the scan proved reachable,
// or a broken walk passes as loudly as a clean tree. [TestTheDeferralScanCanSee]
// feeds the detector synthetic lines containing exactly the shapes this file
// forbids and requires it to find them. The empty result below means something
// only because that control is green.
//
// One limit is disclosed rather than fixed: the gate file is excluded from its
// own scan, so it is a hiding place for a stale referent. It is a test file that
// documents no SDK behaviour, and the exclusion matches one exact path rather
// than a prefix.
package assembly

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// staleReferents are the tickets a forward-looking claim about SDK-side audit
// recording may no longer defer to. Backward citations to any of them are
// correct and are left alone — what this gate forbids is one of them appearing
// as the ticket a *deferral* points at.
//
// The reason differs per entry, and the failure message says which:
//   - AAASM-5731 / AAASM-5681 measured the drop. Neither ever intended to build
//     a sink, and both resolve to finished work.
//   - AAASM-5750 built it. A claim that recording is still Planned under 5750
//     describes shipped behaviour as unbuilt.
var staleReferents = map[string]string{
	"AAASM-5731": "measured the drop and never intended to fix it",
	"AAASM-5681": "measured the drop and never intended to fix it",
	"AAASM-5750": "built the sink; SDK-side recording is no longer deferred",
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
			// An entry that vanished between the directory read and the stat —
			// a build artefact, an editor temp file, a package manager's
			// scratch directory. Returning the error aborts the whole walk,
			// which turns a gate whose verdict is "no findings" into one that
			// produced no verdict at all; the node SDK's equivalent scan hit
			// exactly that in CI. Skipping a file that no longer exists is
			// safe, because it carries no claim. A scan that reaches nothing
			// is the dangerous failure, and TestTheDeferralScanCanSee is what
			// catches that.
			if os.IsNotExist(err) {
				return nil
			}

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
			if os.IsNotExist(err) {
				return nil // removed between the walk and the read
			}

			return err
		}

		sites = append(sites, deferralsInLines(rel, strings.Split(string(body), "\n"))...)

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return sites
}

// deferralsInLines is the detector, split out from the walk so a control can
// drive it over input it constructs rather than over whatever the tree happens
// to contain. A gate whose expected result is "nothing found" is only as good as
// the proof that it can find something.
func deferralsInLines(path string, lines []string) []deferralSite {
	var sites []deferralSite

	for i, line := range lines {
		if !forwardClaim.MatchString(line) {
			continue
		}

		// Extend to the next line only when this line carries no ticket of its
		// own AND does not end a sentence. Without the sentence guard the window
		// pairs a claim with a ticket belonging to the *next* sentence — review
		// produced a real case where an inserted line of forward-looking prose
		// was blamed for a correct backward citation beneath it. There are 30
		// such backward citations in this module.
		window := line
		if !ticketRefRe.MatchString(line) && !endsSentence(line) && i+1 < len(lines) {
			window += "\n" + lines[i+1]
		}

		ticket := ticketRefRe.FindString(window)
		if ticket == "" {
			continue
		}

		sites = append(sites, deferralSite{
			path:   path,
			line:   i + 1,
			ticket: ticket,
			text:   strings.TrimSpace(line),
		})
	}

	return sites
}

func TestNoForwardClaimDefersToAFinishedTicket(t *testing.T) {
	t.Parallel()

	for _, site := range findDeferralSites(t, moduleRoot(t)) {
		if reason, stale := staleReferents[site.ticket]; stale {
			t.Errorf("%s:%d defers to %s, which %s — a forward-looking claim must "+
				"not point at it: %s",
				site.path, site.line, site.ticket, reason, site.text)
		}
	}
}

// TestTheDeferralScanCanSee is the positive control for the assertion above.
//
// That assertion expects to find nothing, and every way of breaking the scan —
// a regex that stops matching, a walk that reaches no files, a window that never
// extends across a wrapped comment — produces exactly the same green. The
// detector is therefore fed input that contains each shape it is supposed to
// catch, and required to catch it. If it stops seeing these, the module-wide
// silence stops meaning anything.
func TestTheDeferralScanCanSee(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		lines  []string
		ticket string
	}{
		{
			name:   "term and ticket on one line",
			lines:  []string{"// recording here is Planned (AAASM-5731), not Observed."},
			ticket: "AAASM-5731",
		},
		{
			name: "term and ticket wrapped onto two lines",
			lines: []string{
				"// Under ADR 0033 section 6 SDK-side recording is Planned",
				"// (AAASM-5750), not Observed.",
			},
			ticket: "AAASM-5750",
		},
		{
			name:   "the termless 'tracked as' shape",
			lines:  []string{"// Supplying a sink that retains it is tracked as AAASM-5681."},
			ticket: "AAASM-5681",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			found := deferralsInLines("synthetic.go", tc.lines)
			if len(found) != 1 || found[0].ticket != tc.ticket {
				t.Fatalf("the detector found %+v in %q; it must find exactly one deferral "+
					"naming %s, or the module-wide empty result proves nothing",
					found, tc.lines, tc.ticket)
			}
			if _, stale := staleReferents[found[0].ticket]; !stale {
				t.Fatalf("%s is not in staleReferents, so this control would not have "+
					"failed the gate even when detected", found[0].ticket)
			}
		})
	}

	// The other direction: a deferral naming a ticket that is genuinely still
	// open must be detected AND permitted. Without this the gate could pass by
	// forbidding every ticket, which would push authors to drop the reference
	// §6 requires rather than to fix the referent.
	open := deferralsInLines("synthetic.go", []string{"// A curated example is Planned (AAASM-9999)."})
	if len(open) != 1 {
		t.Fatalf("the detector missed an unrelated roadmap deferral: %+v", open)
	}
	if _, stale := staleReferents[open[0].ticket]; stale {
		t.Fatalf("%s is treated as stale; an unrelated open ticket must remain legitimate",
			open[0].ticket)
	}
}
