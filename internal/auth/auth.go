// Package auth implements Spotify OAuth 2.0 with PKCE for a public client.
//
// The flow runs a short-lived localhost server to capture the redirect, opens
// the user's browser to authorize, exchanges the code (with the PKCE verifier)
// for a token, and caches it at ~/.config/audiopulse/token.json (0600). A
// persisting token source refreshes and re-saves the token automatically.
//
// No client secret is used or stored — PKCE makes the public Client ID
// sufficient.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/oauth2"

	"audiopulse/internal/config"
)

func oauthConfig(clientID string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: config.RedirectURI,
		Scopes:      config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}
}

// --- token persistence -------------------------------------------------------

func loadToken() (*oauth2.Token, error) {
	path, err := config.TokenPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t oauth2.Token
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func saveToken(t *oauth2.Token) error {
	path, err := config.TokenPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// HasToken reports whether a previously cached token exists.
func HasToken() bool {
	t, err := loadToken()
	return err == nil && t.RefreshToken != ""
}

// persistingSource re-saves the token whenever it is refreshed.
type persistingSource struct {
	base oauth2.TokenSource
	last string
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	t, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if t.AccessToken != p.last {
		_ = saveToken(t)
		p.last = t.AccessToken
	}
	return t, nil
}

// HTTPClient returns an HTTP client that injects (and refreshes) the Spotify
// bearer token. If no token is cached it runs the interactive authorization
// flow first — so callers must invoke this BEFORE entering the alt-screen TUI.
func HTTPClient(ctx context.Context, clientID string) (*http.Client, error) {
	tok, err := loadToken()
	if err != nil {
		tok, err = Authorize(ctx, clientID)
		if err != nil {
			return nil, err
		}
	}
	conf := oauthConfig(clientID)
	src := &persistingSource{base: conf.TokenSource(ctx, tok), last: tok.AccessToken}
	return oauth2.NewClient(ctx, src), nil
}

// --- interactive authorization ----------------------------------------------

// Authorize runs the PKCE flow and returns (and caches) a fresh token.
func Authorize(ctx context.Context, clientID string) (*oauth2.Token, error) {
	conf := oauthConfig(clientID)
	verifier := oauth2.GenerateVerifier()
	state, err := randomState()
	if err != nil {
		return nil, err
	}

	type result struct {
		code string
		err  error
	}
	resCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			writePage(w, "Authorization failed: "+e)
			resCh <- result{err: fmt.Errorf("spotify authorization error: %s", e)}
			return
		}
		if q.Get("state") != state {
			writePage(w, "Authorization failed: state mismatch.")
			resCh <- result{err: errors.New("oauth state mismatch")}
			return
		}
		writePage(w, "AudioPulse is authorized. You can close this tab and return to the terminal.")
		resCh <- result{code: q.Get("code")}
	})

	ln, err := net.Listen("tcp", config.CallbackAddr)
	if err != nil {
		return nil, fmt.Errorf("cannot start callback server on %s (is another instance running?): %w", config.CallbackAddr, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	authURL := conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	fmt.Println("\n♫ AudioPulse needs to connect to your Spotify account.")
	fmt.Println("Opening your browser to authorize…")
	fmt.Printf("\nIf it doesn't open, paste this URL into a browser:\n\n  %s\n\n", authURL)
	_ = openBrowser(authURL)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resCh:
		if res.err != nil {
			return nil, res.err
		}
		tok, err := conf.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
		}
		if err := saveToken(tok); err != nil {
			return nil, fmt.Errorf("saving token: %w", err)
		}
		fmt.Println("Connected to Spotify ✓")
		return tok, nil
	}
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writePage(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8">
<title>AudioPulse</title></head>
<body style="font-family:system-ui;background:#121212;color:#fff;display:flex;
height:100vh;align-items:center;justify-content:center;margin:0">
<div style="text-align:center">
<div style="color:#1DB954;font-size:2rem">&#9834; AudioPulse</div>
<p>%s</p></div></body></html>`, msg)
}

// openBrowser opens url in the user's default browser, including under WSL.
func openBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "cmd", []string{"/c", "start", url}
	case "darwin":
		name, args = "open", []string{url}
	default: // linux / WSL
		switch {
		case lookPath("wslview"):
			name, args = "wslview", []string{url}
		case lookPath("xdg-open"):
			name, args = "xdg-open", []string{url}
		case lookPath("explorer.exe"):
			name, args = "explorer.exe", []string{url}
		default:
			return errors.New("no browser opener found")
		}
	}
	return exec.Command(name, args...).Start()
}

func lookPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
