// Package downloader exports a Spotify library to local audio files by driving
// the external spotDL tool (https://github.com/spotDL/spotify-downloader) as a
// subprocess. spotDL reads track metadata and pulls the matching audio from
// YouTube, embedding the original cover art and tags.
//
// AudioPulse already knows every track URI (it paginates playlists / Liked
// Songs), so it feeds spotDL explicit URIs in batches and reports aggregate
// progress, rather than handing spotDL whole playlists. spotDL skips files that
// already exist, so an interrupted export is resumable by re-running it.
package downloader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// batchSize bounds how many URIs go to a single spotdl invocation — small
	// enough that a killed (stalled) batch loses little, large enough to amortize
	// spotdl's startup cost.
	batchSize = 40
	// stallTimeout kills a batch that produces no output for this long (a hung,
	// throttled download); its unfinished tracks have no file, so the next run
	// retries them.
	stallTimeout = 3 * time.Minute
	// logName / reportName are written into the output dir for diagnosis.
	logName    = "_export.log"
	reportName = "_export-failures.txt"
)

// Find locates the spotdl binary on PATH or in a pipx install location.
func Find() (string, bool) {
	if p, err := exec.LookPath("spotdl"); err == nil {
		return p, true
	}
	return "", false
}

// Available reports whether spotdl is installed.
func Available() bool {
	_, ok := Find()
	return ok
}

// Progress is an aggregate snapshot of an export run.
type Progress struct {
	Total    int    // tracks requested
	Done     int    // downloaded
	Skipped  int    // already present
	Failed   int    // no match / error
	Current  string // most recent track line
	Finished bool   // the run has ended
	Err      error  // fatal error (spotdl missing, etc.)
}

// Processed is how many tracks spotdl has reported a result for.
func (p Progress) Processed() int { return p.Done + p.Skipped + p.Failed }

// Export downloads uris to outDir via spotdl, streaming Progress updates on the
// returned channel (the final one has Finished=true). Cancel via ctx. The
// channel is closed when the run ends.
func Export(ctx context.Context, uris []string, outDir string) <-chan Progress {
	ch := make(chan Progress, 32)
	go func() {
		defer close(ch)
		run(ctx, uris, outDir, ch)
	}()
	return ch
}

func run(ctx context.Context, uris []string, outDir string, ch chan<- Progress) {
	bin, ok := Find()
	if !ok {
		ch <- Progress{Total: len(uris), Finished: true, Err: errSpotdlMissing}
		return
	}

	lg, closeLog := openLog(outDir)
	defer closeLog()
	lg.printf("=== AudioPulse export: %d tracks → %s ===", len(uris), outDir)

	p := Progress{Total: len(uris)}
	t := newTally() // dedupe across all batches
	output := filepath.Join(outDir, "{artist}", "{album}", "{title}.{output-ext}")

	for start := 0; start < len(uris) && ctx.Err() == nil; start += batchSize {
		end := start + batchSize
		if end > len(uris) {
			end = len(uris)
		}
		lg.printf("--- batch %d-%d of %d ---", start+1, end, len(uris))

		args := append([]string{"download"}, uris[start:end]...)
		args = append(args, "--output", output, "--format", "mp3", "--threads", "4")

		bctx, bcancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(bctx, bin, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			bcancel()
			p.Err = err
			break
		}
		cmd.Stderr = cmd.Stdout // fold stderr into the same stream we parse
		if err := cmd.Start(); err != nil {
			bcancel()
			p.Err = err
			break
		}

		// Kill the batch if it goes silent (a hung, throttled download).
		ping := make(chan struct{}, 1)
		go watchdog(bctx, bcancel, ping, lg)

		scanLines(stdout, func(line string) {
			select {
			case ping <- struct{}{}: // keep the watchdog alive
			default:
			}
			lg.println(line)
			if name, changed := t.add(line); changed {
				p.Done, p.Skipped, p.Failed = t.done(), t.skipped(), t.failed()
				p.Current = name
				ch <- p // emit when a distinct song's outcome changes
			}
		})
		_ = cmd.Wait()
		bcancel()
	}

	writeFailures(outDir, t.failures())
	p.Finished = true
	if ctx.Err() != nil && p.Err == nil {
		p.Err = ctx.Err()
	}
	ch <- p
}

// watchdog cancels a batch that emits no output for stallTimeout.
func watchdog(ctx context.Context, cancel context.CancelFunc, ping <-chan struct{}, lg *logger) {
	t := time.NewTimer(stallTimeout)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(stallTimeout)
		case <-t.C:
			lg.printf("[audiopulse] no output for %s — killing stalled batch (retried next run)", stallTimeout)
			cancel()
			return
		}
	}
}

// logger is a small concurrency-safe writer (the watchdog and reader both log).
type logger struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *logger) println(s string) { l.mu.Lock(); fmt.Fprintln(l.w, s); l.mu.Unlock() }
func (l *logger) printf(f string, a ...any) {
	l.mu.Lock()
	fmt.Fprintf(l.w, f+"\n", a...)
	l.mu.Unlock()
}

func openLog(outDir string) (*logger, func()) {
	f, err := os.Create(filepath.Join(outDir, logName))
	if err != nil {
		return &logger{w: io.Discard}, func() {}
	}
	bw := bufio.NewWriter(f)
	return &logger{w: bw}, func() { bw.Flush(); f.Close() }
}

// writeFailures records the tracks that couldn't be downloaded, so the user has
// a concrete list to source elsewhere. Removes a stale report when none failed.
func writeFailures(outDir string, failures []string) {
	path := filepath.Join(outDir, reportName)
	if len(failures) == 0 {
		_ = os.Remove(path)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %d track(s) could not be downloaded by spotDL.\n", len(failures))
	b.WriteString("# Usually Spotify-exclusive recordings (Singles/Sessions/Live) or odd\n")
	b.WriteString("# titles that aren't on YouTube. Source these elsewhere if you want them.\n\n")
	for _, f := range failures {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

// failureName extracts a human label for a failed track from its spotdl line.
func failureName(line string) string {
	if m := reNoResults.FindStringSubmatch(line); m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(line)
}

func scanLines(r io.Reader, fn func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fn(sc.Text())
	}
}

var (
	reDownloaded = regexp.MustCompile(`Downloaded "([^"]+)"`)
	reSkipping   = regexp.MustCompile(`Skipping ([^\(]+?)(?: \(|$)`)
	reError      = regexp.MustCompile(`(?i)(error|no results found)[^:]*:?\s*(.*)`)
	reNoResults  = regexp.MustCompile(`No results found for song:\s*(.*)`)
)

// classify maps one spotdl output line to a per-song result and the song's name.
// kind is "" for non-result lines.
func classify(line string) (kind, name string) {
	switch {
	case reDownloaded.MatchString(line):
		return "done", firstGroup(reDownloaded, line)
	case reSkipping.MatchString(line):
		return "skip", strings.TrimSpace(firstGroup(reSkipping, line))
	case isErrorLine(line):
		return "fail", failureName(line)
	}
	return "", ""
}

const (
	rankFail = 1
	rankSkip = 2
	rankDone = 3
)

func rankOf(kind string) int {
	switch kind {
	case "done":
		return rankDone
	case "skip":
		return rankSkip
	case "fail":
		return rankFail
	}
	return 0
}

// tally dedupes spotdl results by song name, so retries (spotdl prints an error
// per retry) and duplicate URIs (the same song reached via several playlists)
// aren't counted multiple times. Each song keeps its best outcome:
// downloaded > skipped/already-have > failed.
type tally struct {
	rank   map[string]int
	counts [4]int
}

func newTally() *tally { return &tally{rank: make(map[string]int)} }

// add records a line and returns the song name plus whether the totals changed.
func (t *tally) add(line string) (string, bool) {
	kind, name := classify(line)
	if kind == "" {
		return "", false
	}
	nr := rankOf(kind)
	if or := t.rank[name]; nr > or {
		if or != 0 {
			t.counts[or]--
		}
		t.counts[nr]++
		t.rank[name] = nr
		return name, true
	}
	return name, false
}

func (t *tally) done() int    { return t.counts[rankDone] }
func (t *tally) skipped() int { return t.counts[rankSkip] }
func (t *tally) failed() int  { return t.counts[rankFail] }

// failures returns the distinct song names whose best outcome was a failure.
func (t *tally) failures() []string {
	var out []string
	for name, r := range t.rank {
		if r == rankFail {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func isErrorLine(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "no results found") ||
		strings.Contains(l, "lookuperror") ||
		strings.HasPrefix(strings.TrimSpace(line), "Error")
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

var errSpotdlMissing = fmt.Errorf("spotdl is not installed — run 'make spotdl'")
