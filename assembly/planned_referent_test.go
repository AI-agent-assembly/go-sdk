// AAASM-5750 — the ADR 0033 §6 `Planned` term must reference the ticket that
// will build the capability, not the one that measured its absence.
//
// §6 scopes `Planned` to "decided but not implemented — a ticket reference; no
// capability claim." A reference to a ticket that never intended to deliver the
// capability goes stale the moment that ticket closes: the term still reads as
// a live commitment while the reference points at finished work. Nothing
// mechanical catches that, because the referent lives in a comment, and a
// comment is the one artifact in a source file with no check on it at all.
//
// This is that check. It is deliberately a source scan rather than a review
// convention: the previous referent was corrected by hand in five places here
// and stayed correct only until the next edit.
//
// The floor is a ratchet, not a transcription — it was measured from the tree
// at the time this was written, and exists so that deleting sites fails rather
// than silently emptying the scan. An empty scan and a clean scan are otherwise
// the same result.
package assembly

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// capabilityReferent is the ticket that owns building the SDK-side audit sink.
const capabilityReferent = "AAASM-5750"

// plannedReferentFloor is the number of §6 `Planned` sites carrying a ticket
// reference when this gate was written. Fewer means sites were removed without
// this gate being revisited, which would leave it passing over nothing.
const plannedReferentFloor = 5

var (
	plannedTerm  = regexp.MustCompile(`\bPlanned\b`)
	ticketRefRe  = regexp.MustCompile(`AAASM-\d+`)
	scannedFiles = regexp.MustCompile(`\.(go|md)$`)
)

// moduleRoot walks up from the test's working directory to the directory
// holding go.mod. Resolving it here rather than assuming a relative path keeps
// the scan module-wide regardless of which package `go test` is invoked from.
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

type plannedSite struct {
	path   string
	line   int
	ticket string
	text   string
}

// findPlannedSites returns every line where the §6 term and a ticket reference
// are co-located. A `Planned` with no ticket on it is prose continuation, not a
// referent, and is not a site — tool_wrapper.go's "which is what Planned names"
// is the case that distinction exists for.
func findPlannedSites(t *testing.T, root string) []plannedSite {
	t.Helper()

	var sites []plannedSite

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

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		for i, line := range strings.Split(string(body), "\n") {
			if !plannedTerm.MatchString(line) {
				continue
			}

			ticket := ticketRefRe.FindString(line)
			if ticket == "" {
				continue
			}

			sites = append(sites, plannedSite{
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

func TestPlannedReferencesTheCapabilityTicket(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	sites := findPlannedSites(t, root)

	// Positive control on the scan itself. Without it, a walk that reached no
	// files — a broken root, a changed extension set — reports the same clean
	// result as a tree with every referent correct.
	if len(sites) < plannedReferentFloor {
		t.Fatalf("scan found %d §6 Planned referent sites under %s, floor is %d; "+
			"either sites were removed without revisiting this gate, or the scan "+
			"stopped reaching them and is passing over nothing",
			len(sites), root, plannedReferentFloor)
	}

	for _, site := range sites {
		if site.ticket != capabilityReferent {
			t.Errorf("%s:%d references %s as the §6 Planned referent, want %s "+
				"(the ticket that builds the sink, not one that measured its "+
				"absence): %s",
				site.path, site.line, site.ticket, capabilityReferent, site.text)
		}
	}
}
