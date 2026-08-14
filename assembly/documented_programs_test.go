package assembly

// Drift gate that EXECUTES every complete Go program published in this repo's
// Markdown (AAASM-5662).
//
// WHY EXECUTION, AND NOT A GREP
//
// The sibling gate in quickstart_claim_bindings_test.go binds prose to controls,
// and says so in its own header: "It does not compile or execute a documented
// snippet. Binding a claim does not make it true." That hole was not
// theoretical. Two published `package main` programs called
// `WrapTools(tools, nil)` and then narrated the result of the call — one with a
// trailing `// result: hello, governance` comment on the line after the
// log.Fatalf that the call actually reaches. Executed, both exited 1, and a
// third failed to compile on an undefined identifier. Every text-level gate in
// the repo was green throughout, because a string can be present and false at
// the same time.
//
// So this gate does the one thing no amount of reading can do: it runs them.
//
// WHAT IT PROVES
//
//  1. The scan finds the programs it claims to gate — asserted as a SET of
//     files, not a count, and cross-checked against a synthetic fixture whose
//     block count is known independently of the extractor.
//  2. Each program COMPILES (go vet), so an undefined identifier is caught.
//  3. Each program RUNS TO COMPLETION with exit status 0.
//  4. Where a program annotates a line `// result: X`, the run's output actually
//     contains X — so a comment cannot narrate an outcome the program never
//     reaches.
//  5. The runner reports failure when a program fails. Proven here by feeding it
//     three synthetic programs that fail in the three ways that matter (compile
//     error, non-zero exit, absent output) and requiring each to be caught.
//
// WHAT IT DOES NOT PROVE
//
// No documented program reaches a live gateway. The default pure-Go build links
// no native transport, so `Init` returns ErrSidecarUnavailable on every option
// set (assembly/sidecar.go: connectToLocalSidecar returns an error
// unconditionally) and no gRPC/HTTP call leaves the process. What runs here is
// the wrapper's decision path against an in-process GovernanceClient — an
// *Evaluated* decision that reaches *Denied before execution*, per ADR 0033 §6.
// A gateway-backed Check is Unmeasured by this gate and belongs to the e2e
// repos.
//
// The programs are executed in a throwaway module under t.TempDir() whose go.mod
// is this repo's own with a `replace` onto the checkout, so no network fetch and
// no resolution ambiguity: the documented program is built against the working
// tree, which is the only version whose behavior is under test.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoRoot is the checkout root relative to this package's test working
// directory, matching quickstart_claim_bindings_test.go's "../docs/…".
const repoRoot = ".."

// docProgramTrees are walked recursively for Markdown. The checkout root is
// scanned separately and shallowly, for README.md and its siblings — walking it
// recursively would descend into website/node_modules and native/target.
var docProgramTrees = []string{"docs"}

// expectedProgramFiles are the files known to publish a complete program.
//
// Asserted as a SET the scan must CONTAIN, not as a count. A count is satisfied
// by any four programs, so moving a program from one page to another — or
// deleting one and adding one elsewhere — would keep a count green while the
// page a reader lands on stops being covered.
var expectedProgramFiles = []string{
	"docs/_index.md",
	"docs/guides/govern-an-agents-tools.md",
	"docs/quick-start.md",
}

var (
	goFenceOpen  = regexp.MustCompile("^\\s*```go\\s*$")
	goFenceClose = regexp.MustCompile("^\\s*```\\s*$")
	// resultComment captures a trailing line comment that narrates output:
	// `// result: X`. Multi-line, because the anchor has to be end-of-LINE — with
	// the default anchoring only the last line of a program could ever match, and
	// this pattern would have found nothing while reporting nothing wrong. A bare
	// `// X` is deliberately not matched: most trailing comments are not output
	// claims, and asserting over them would make the gate noisy rather than
	// stricter.
	resultComment = regexp.MustCompile(`(?m)//\s*result:\s*(\S.*?)\s*$`)
)

// docProgram is one complete `package main` published in Markdown.
type docProgram struct {
	// file is repo-relative, e.g. "docs/quick-start.md".
	file string
	// openLine is the 1-based line of the opening ```go fence.
	openLine int
	source   string
}

func (p docProgram) String() string {
	return fmt.Sprintf("%s:%d", p.file, p.openLine)
}

// dirName is a filesystem-safe package directory name for this program.
func (p docProgram) dirName() string {
	safe := strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(p.file)
	return fmt.Sprintf("p%s_%d", safe, p.openLine)
}

// extractGoPrograms returns every fenced ```go block in body that opens with
// `package main`. Split out from the file walk so the extractor can be exercised
// against a fixture whose answer is known without running the extractor.
func extractGoPrograms(file, body string) []docProgram {
	var programs []docProgram
	var current []string
	openLine := 0
	for index, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		lineNumber := index + 1
		if current == nil {
			if goFenceOpen.MatchString(line) {
				current = []string{}
				openLine = lineNumber
			}
			continue
		}
		if goFenceClose.MatchString(line) {
			source := strings.Join(current, "\n")
			if strings.HasPrefix(strings.TrimSpace(source), "package main") {
				programs = append(programs, docProgram{file: file, openLine: openLine, source: source + "\n"})
			}
			current = nil
			continue
		}
		current = append(current, line)
	}
	return programs
}

// discoverDocPrograms walks the documentation for complete programs.
func discoverDocPrograms(t *testing.T) []docProgram {
	t.Helper()

	var programs []docProgram
	for _, path := range markdownFiles(t) {
		body, err := os.ReadFile(filepath.Join(repoRoot, path))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		programs = append(programs, extractGoPrograms(path, string(body))...)
	}
	sort.Slice(programs, func(i, j int) bool {
		if programs[i].file != programs[j].file {
			return programs[i].file < programs[j].file
		}
		return programs[i].openLine < programs[j].openLine
	})
	return programs
}

// markdownFiles returns every repo-relative Markdown path this gate owns: the
// checkout root's own files, plus everything under docProgramTrees.
func markdownFiles(t *testing.T) []string {
	t.Helper()

	var paths []string
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		t.Fatalf("reading the checkout root: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			paths = append(paths, entry.Name())
		}
	}

	for _, tree := range docProgramTrees {
		walkErr := filepath.WalkDir(filepath.Join(repoRoot, tree), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			relative, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				return relErr
			}
			paths = append(paths, filepath.ToSlash(relative))
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walking %s: %v", tree, walkErr)
		}
	}
	sort.Strings(paths)
	return paths
}

// docProgramModule materialises a throwaway module that builds against this
// checkout. The repo's own go.mod supplies the require graph and its go.sum the
// checksums, so `go run` resolves entirely from the module cache: no network,
// and no chance of building the documented program against a PUBLISHED SDK
// version rather than the working tree the change is in.
func docProgramModule(t *testing.T) string {
	t.Helper()

	checkout, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("resolving the checkout path: %v", err)
	}

	goMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	goSum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("reading go.sum: %v", err)
	}

	lines := strings.Split(string(goMod), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "module ") {
			lines[index] = "module aaasmdocumentedprograms"
			break
		}
	}
	rewritten := strings.Join(lines, "\n") +
		"\n\nrequire " + moduleImportPath + " v0.0.0\n" +
		"\nreplace " + moduleImportPath + " => " + checkout + "\n"

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), rewritten)
	writeFile(t, filepath.Join(dir, "go.sum"), string(goSum))
	return dir
}

// moduleImportPath is this module's path. Kept as one constant because it
// appears in both the require and the replace directive above.
const moduleImportPath = "github.com/ai-agent-assembly/go-sdk"

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// stage writes a program into the module as its own main package and returns the
// package's `./…` path.
func stage(t *testing.T, module string, program docProgram) string {
	t.Helper()
	dir := filepath.Join(module, program.dirName())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	writeFile(t, filepath.Join(dir, "main.go"), program.source)
	return "./" + program.dirName()
}

// runGo runs one go subcommand in the throwaway module and returns its combined
// output and exit code. An exec failure that is NOT a non-zero exit (a missing
// toolchain, say) fails the test rather than being reported as an exit code: a
// gate that cannot run its subject must say so, not return a number that reads
// like a verdict.
func runGo(t *testing.T, module string, args ...string) (string, int) {
	t.Helper()
	command := exec.Command("go", args...)
	command.Dir = module
	// GOFLAGS is cleared so an ambient -mod=vendor / -mod=readonly from the
	// caller's environment cannot change what this module resolves.
	//
	// AA_GATEWAY_URL is pinned because a program that reaches Init WITHOUT an
	// explicit gateway URL makes resolveGatewayURL probe :7391 and then shell out
	// to `aasm start --mode local --foreground`. On a machine with the CLI
	// installed that starts a real gateway daemon as a grandchild of `go test` —
	// observed here, where it bound :7391 for long enough to make three unrelated
	// tests in this package see a reachable gateway and fail. A gate must not
	// start a service on the host it runs on. Every documented program passes its
	// own WithGatewayURL, so this changes nothing they exercise; it stops a future
	// one from spawning a daemon.
	command.Env = append(os.Environ(),
		"GOFLAGS=",
		"AA_GATEWAY_URL=https://gateway.invalid",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(output), exitErr.ExitCode()
	}
	t.Fatalf("could not run `go %s`: %v\n%s", strings.Join(args, " "), err, output)
	return "", 0
}

// TestDocumentedProgramScanFindsWhatItGates is the positive control. An empty
// scan and a clean run are indistinguishable without it.
func TestDocumentedProgramScanFindsWhatItGates(t *testing.T) {
	t.Run("TheExtractorFindsBlocksInAFixtureWithAKnownAnswer", func(t *testing.T) {
		// Three fences, of which two open with `package main`. Written out here
		// so the expected answer does not come from the extractor itself.
		fixture := strings.Join([]string{
			"# heading",
			"```go",
			"package main",
			"func main() {}",
			"```",
			"prose",
			"```go",
			"governed := assembly.WrapTools(myTools, nil)",
			"```",
			"```go",
			"package main",
			"// second",
			"```",
			"```bash",
			"package main",
			"```",
		}, "\n")

		found := extractGoPrograms("fixture.md", fixture)
		if len(found) != 2 {
			t.Fatalf("the extractor found %d programs in a fixture holding exactly 2: %v", len(found), found)
		}
		if found[0].openLine != 2 || found[1].openLine != 10 {
			t.Fatalf("the extractor mislocated the fences: %v", found)
		}
		if !strings.Contains(found[1].source, "// second") {
			t.Fatalf("the extractor lost the body of the second program: %q", found[1].source)
		}
	})

	t.Run("EveryFileKnownToPublishAProgramIsCovered", func(t *testing.T) {
		covered := map[string]bool{}
		for _, program := range discoverDocPrograms(t) {
			covered[program.file] = true
		}
		for _, file := range expectedProgramFiles {
			if !covered[file] {
				t.Errorf("%s publishes a complete program that this gate no longer sees.\n"+
					"Files currently covered: %v\n"+
					"Either the program moved (update expectedProgramFiles) or the scan stopped "+
					"reaching that directory — the second is the failure this assertion exists for.",
					file, sortedKeys(covered))
			}
		}
	})

	t.Run("TheScanIsNotEmpty", func(t *testing.T) {
		if n := len(discoverDocPrograms(t)); n < len(expectedProgramFiles) {
			t.Fatalf("the scan found %d documented programs, fewer than the %d files known to "+
				"publish one", n, len(expectedProgramFiles))
		}
	})
}

// TestEveryDocumentedProgramCompiles catches the failure a reader hits first:
// a published program that does not build. `go vet` type-checks, so an
// undefined identifier fails here.
func TestEveryDocumentedProgramCompiles(t *testing.T) {
	module := docProgramModule(t)
	for _, program := range discoverDocPrograms(t) {
		packagePath := stage(t, module, program)
		output, code := runGo(t, module, "vet", packagePath)
		if code != 0 {
			t.Errorf("the program published at %s does not compile (go vet exit %d):\n%s",
				program, code, output)
		}
	}
}

// TestEveryDocumentedProgramRunsToCompletion executes each program and requires
// exit status 0. This is the assertion the ticket turns on: the two programs it
// was raised for exited 1 at Init, one line before the tool call their prose
// described.
func TestEveryDocumentedProgramRunsToCompletion(t *testing.T) {
	module := docProgramModule(t)
	for _, program := range discoverDocPrograms(t) {
		packagePath := stage(t, module, program)
		output, code := runGo(t, module, "run", packagePath)
		if code != 0 {
			t.Errorf("the program published at %s exits %d rather than running to completion:\n%s\n"+
				"Correct the program, or move the part that needs a live runtime out of a "+
				"`package main` block and say at that site what it requires.",
				program, code, output)
		}
	}
}

// TestDocumentedOutputCommentsMatchTheRun binds a narrating comment to the run.
// The defect this gate was raised for was exactly a `// result: hello,
// governance` comment on an unreachable line.
func TestDocumentedOutputCommentsMatchTheRun(t *testing.T) {
	module := docProgramModule(t)
	asserted := 0
	for _, program := range discoverDocPrograms(t) {
		expected := resultComment.FindAllStringSubmatch(program.source, -1)
		if len(expected) == 0 {
			continue
		}
		packagePath := stage(t, module, program)
		output, code := runGo(t, module, "run", packagePath)
		if code != 0 {
			// Reported by TestEveryDocumentedProgramRunsToCompletion; nothing to
			// add here beyond not asserting over the output of a dead program.
			continue
		}
		for _, match := range expected {
			asserted++
			if !strings.Contains(output, match[1]) {
				t.Errorf("the program published at %s annotates a line `// result: %s`, but running "+
					"it produced:\n%s\nA comment that narrates output the run never produces is the "+
					"defect this gate exists for.", program, match[1], output)
			}
		}
	}
	if asserted == 0 {
		t.Errorf("no documented program carries a `// result: …` annotation any more, so this " +
			"assertion checked nothing. Restore one, or delete this test — a gate that silently " +
			"stops having a subject is worse than no gate.")
	}
}

// TestTheProgramRunnerReportsFailure is the negative control for the runner
// itself. Every assertion above is of the form "exit code == 0"; if the runner
// could not observe a failure, they would pass on any documentation at all.
//
// The three synthetic programs fail in the three ways a documented program
// actually fails, and each must be caught by the same code path the real
// programs go through.
func TestTheProgramRunnerReportsFailure(t *testing.T) {
	module := docProgramModule(t)

	t.Run("AProgramThatDoesNotCompileIsReported", func(t *testing.T) {
		program := docProgram{
			file:     "synthetic/does-not-compile.md",
			openLine: 1,
			source:   "package main\n\nfunc main() {\n\t_ = undefinedIdentifier\n}\n",
		}
		output, code := runGo(t, module, "vet", stage(t, module, program))
		if code == 0 {
			t.Fatalf("go vet accepted a program referencing an undefined identifier:\n%s", output)
		}
		if !strings.Contains(output, "undefinedIdentifier") {
			t.Fatalf("the compile failure was reported without naming the cause:\n%s", output)
		}
	})

	t.Run("AProgramThatExitsNonZeroIsReported", func(t *testing.T) {
		// The shape of the defect: log.Fatalf on the error the SDK actually
		// returns, one line before the call the prose describes.
		program := docProgram{
			file:     "synthetic/exits-nonzero.md",
			openLine: 1,
			source: "package main\n\nimport (\n\t\"context\"\n\t\"log\"\n\n\t\"" + moduleImportPath +
				"/assembly\"\n)\n\nfunc main() {\n\tctx := context.Background()\n\t" +
				"_, err := assembly.Init(ctx, assembly.WithGatewayURL(\"https://gateway.example.com\"))\n" +
				"\tif err != nil {\n\t\tlog.Fatalf(\"init: %v\", err)\n\t}\n" +
				"\tlog.Println(\"unreachable\")\n}\n",
		}
		output, code := runGo(t, module, "run", stage(t, module, program))
		if code == 0 {
			t.Fatalf("a program that log.Fatalf's on Init's error reported success:\n%s", output)
		}
		if !strings.Contains(output, "sidecar unavailable") {
			t.Fatalf("the run failed, but not for the documented reason:\n%s", output)
		}
	})

	t.Run("AResultCommentTheRunNeverProducesIsReported", func(t *testing.T) {
		program := docProgram{
			file:     "synthetic/wrong-result.md",
			openLine: 1,
			source: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t" +
				"fmt.Println(\"denied\") // result: hello, governance\n}\n",
		}
		expected := resultComment.FindStringSubmatch(program.source)
		if expected == nil {
			t.Fatal("the result-comment pattern no longer matches the form the docs use")
		}
		output, code := runGo(t, module, "run", stage(t, module, program))
		if code != 0 {
			t.Fatalf("the synthetic program did not run: %s", output)
		}
		if strings.Contains(output, expected[1]) {
			t.Fatalf("a comment claiming %q was not distinguished from output that lacks it:\n%s",
				expected[1], output)
		}
	})
}

// TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns pins the Init outcome the
// quick-start and the guide now describe, in-process rather than through a
// subprocess.
//
// The pages used to say Init succeeds "once a gateway is reachable", and each
// published program acted on that by calling log.Fatalf on the error. It is not
// what the default build does: newAssembly wires the pure-Go binding, whose
// connect reports the runtime unavailable, so boot falls through to
// connectToLocalSidecar — which returns an error unconditionally (sidecar.go).
// Supplying WithSidecarAddress does not change that on this build, which is the
// half a "without one, Init returns ErrSidecarUnavailable" wording concealed.
//
// Deliberately asserted through the exported Init, not by reading sidecar.go: a
// control that restated the implementation would move with it.
func TestTheDocumentedInitOutcomeIsTheOneTheSDKReturns(t *testing.T) {
	ctx := WithAgentID(context.Background(), "documented-program-agent")

	t.Run("InitReportsTheRuntimeUnavailableWithNoSidecarOption", func(t *testing.T) {
		a, err := Init(ctx, WithGatewayURL("https://gateway.example.com"), WithAPIKey("..."))
		if !errors.Is(err, ErrSidecarUnavailable) {
			t.Fatalf("Init returned (%v, %v); the documented programs branch on ErrSidecarUnavailable", a, err)
		}
	})

	t.Run("InitReportsTheRuntimeUnavailableWithASidecarAddressToo", func(t *testing.T) {
		a, err := Init(ctx,
			WithGatewayURL("https://gateway.example.com"),
			WithSidecarAddress("127.0.0.1:50051"),
		)
		if !errors.Is(err, ErrSidecarUnavailable) {
			t.Fatalf("Init returned (%v, %v) with a sidecar address; the pages say the address does "+
				"not change the outcome on the default build, so one of the two is now wrong", a, err)
		}
	})
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
