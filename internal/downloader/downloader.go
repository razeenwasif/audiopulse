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
	"os/exec"
	"regexp"
	"strings"
)

// batchSize bounds how many URIs go to a single spotdl invocation, to stay well
// under command-line length limits on large libraries.
const batchSize = 75

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

	p := Progress{Total: len(uris)}
	output := outDir + "/{artist}/{album}/{title}.{output-ext}"

	for start := 0; start < len(uris); start += batchSize {
		if ctx.Err() != nil {
			break
		}
		end := start + batchSize
		if end > len(uris) {
			end = len(uris)
		}
		args := append([]string{"download"}, uris[start:end]...)
		args = append(args, "--output", output, "--format", "mp3", "--threads", "4")

		cmd := exec.CommandContext(ctx, bin, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			p.Err = err
			break
		}
		cmd.Stderr = cmd.Stdout // fold stderr into the same stream we parse
		if err := cmd.Start(); err != nil {
			p.Err = err
			break
		}

		scanLines(stdout, func(line string) {
			if applyLine(&p, line) {
				ch <- p // emit on every per-track result
			}
		})
		_ = cmd.Wait()
	}

	p.Finished = true
	if ctx.Err() != nil && p.Err == nil {
		p.Err = ctx.Err()
	}
	ch <- p
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
)

// applyLine updates p from one spotdl output line; it returns true when the line
// represented a per-track result worth emitting.
func applyLine(p *Progress, line string) bool {
	switch {
	case reDownloaded.MatchString(line):
		p.Done++
		p.Current = firstGroup(reDownloaded, line)
		return true
	case reSkipping.MatchString(line):
		p.Skipped++
		p.Current = strings.TrimSpace(firstGroup(reSkipping, line))
		return true
	case isErrorLine(line):
		p.Failed++
		if m := reError.FindStringSubmatch(line); m != nil && strings.TrimSpace(m[2]) != "" {
			p.Current = strings.TrimSpace(m[2])
		}
		return true
	}
	return false
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
