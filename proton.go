package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	bridge "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/Proton-API-Bridge/common"
	"github.com/henrybear327/go-proton-api"
)

// ---------------------------------------------------------------------------
// Proton Drive upload side, on top of Proton-API-Bridge, which handles SRP
// login, 2FA and the end-to-end encryption of everything it uploads.
//
// Sign-in happens in the browser, the same way the Google side does: this app
// serves a one-page form on 127.0.0.1 and opens it in the browser. The
// credentials are used to sign in once and are then dropped: nothing about the
// account, the session or the keys is written anywhere, on disk or elsewhere.
// ---------------------------------------------------------------------------

type protonClient struct {
	ctx        context.Context
	pd         *bridge.ProtonDrive
	folderSize map[string]*ProtonEntry // per-folder listing cache, keyed by link ID
}

type ProtonEntry struct {
	Name     string
	IsFolder bool
	Size     int64
}

// newProtonDriveConfig returns a bridge configuration that never writes a
// credential cache and keeps its (download-only) data folder in the system
// temporary directory, away from the user's files.
func newProtonDriveConfig(appVersion string) *common.Config {
	config := common.NewConfigWithDefaultValues()
	config.AppVersion = appVersion
	config.UserAgent = "gdrive2proton/1.0.0 (Google Drive to Proton Drive migration)"
	config.CredentialCacheFile = "" // empty: no session is ever written to disk
	config.DataFolderName = filepath.Join(os.TempDir(), "gdrive2proton-proton-data")
	return config
}

// protonSignIn performs one sign-in attempt with the given credentials.
//
// Proton requires every third-party client to identify itself with the
// x-pm-appversion header in the form external-drive-{name}@{semver}-{channel};
// unregistered or empty strings are rejected with code 5002.
func protonSignIn(ctx context.Context, appVersion, username, password, twofa string) (*bridge.ProtonDrive, error) {
	config := newProtonDriveConfig(appVersion)
	config.FirstLoginCredential = &common.FirstLoginCredentialData{
		Username:        username,
		Password:        password,
		TwoFA:           twofa,
		MailboxPassword: "",
	}
	pd, _, err := bridge.NewProtonDrive(ctx, config,
		func(auth proton.Auth) { // new auth issued mid-session; tokens stay in memory
		},
		func() { // de-authed; nothing is cached, nothing to clean up
		})
	if err != nil {
		return nil, err
	}
	return pd, nil
}

const protonLoginPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>gdrive2proton — Proton sign-in</title>
<style>
body{font-family:system-ui,sans-serif;background:#f4f4f5;display:flex;justify-content:center;padding-top:8vh}
.card{background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.12);padding:2rem 2.5rem;width:22rem}
h1{font-size:1.1rem;margin:0 0 .3rem}p{color:#555;font-size:.85rem;margin:.2rem 0 1rem}
label{display:block;font-size:.85rem;margin:.7rem 0 .2rem;color:#333}
input{width:100%%;box-sizing:border-box;padding:.5rem;border:1px solid #ccc;border-radius:6px;font-size:.95rem}
button{margin-top:1.2rem;width:100%%;padding:.6rem;border:0;border-radius:6px;background:#6d4aff;color:#fff;font-size:1rem;cursor:pointer}
.err{color:#b3261e;font-size:.85rem;margin-top:.8rem}
.note{color:#888;font-size:.75rem;margin-top:1.2rem;border-top:1px solid #eee;padding-top:.8rem}
</style></head><body><div class="card">
<h1>Sign in to Proton Drive</h1>
<p>This page is served by gdrive2proton on your own computer. The password goes straight to Proton's servers to sign in and is not stored anywhere.</p>
<form method="post" action="/migrate/proton">
<label>Username or email</label><input name="username" autofocus required value="%[1]s">
<label>Password</label><input name="password" type="password" required>
<label>Two-factor code %[2]s</label><input name="twofa" autocomplete="one-time-code" inputmode="numeric">
<button>Sign in</button>
%[3]s
</form>
<p class="note">Nothing is saved: no session, no tokens, no keys. Each run signs in fresh.</p>
</div></body></html>`

// newProtonClient runs the browser sign-in and returns a signed-in client.
// The credentials exist only inside the local login page's request and are
// dropped when the sign-in completes.
func newProtonClient(ctx context.Context, appVersion string) (*protonClient, error) {
	var (
		pdCh   = make(chan *bridge.ProtonDrive, 1)
		srvErr = make(chan error, 1)
	)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	loginURL := fmt.Sprintf("http://127.0.0.1:%d/", l.Addr().(*net.TCPAddr).Port)

	var srv *http.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, pageProtonLogin("", ""))
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		r.ParseForm()
		username := strings.TrimSpace(r.PostFormValue("username"))
		password := r.PostFormValue("password")
		twofa := strings.TrimSpace(r.PostFormValue("twofa"))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pd, err := protonSignIn(r.Context(), appVersion, username, password, twofa)
		if err != nil {
			msg := "Sign-in failed. Check the username and password."
			if strings.Contains(strings.ToLower(err.Error()), "2fa") {
				msg = "This account has two-factor authentication: enter the 6-digit code from your authenticator app."
			} else if strings.Contains(strings.ToLower(err.Error()), "password") {
				msg = "Sign-in failed: wrong username or password."
			}
			fmt.Fprint(w, pageProtonLogin(msg, html.EscapeString(username)))
			return
		}
		// Success: show the confirmation, give the tab a moment, shut down.
		fmt.Fprint(w, `<!doctype html><html><body style="font-family:system-ui,sans-serif;padding-top:12vh;text-align:center">
<h2>Signed in to Proton Drive ✓</h2><p>You can close this tab and return to the terminal.</p></body></html>`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() { time.Sleep(time.Second); srv.Close() }()
		pdCh <- pd
	})
	srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { srvErr <- srv.Serve(l) }()

	fmt.Println("\nOpening your browser to sign in to Proton Drive.")
	fmt.Println("If it does not open, paste this URL into a browser:")
	fmt.Println()
	fmt.Println(loginURL)
	fmt.Println()
	if err := openBrowser(loginURL); err != nil {
		fmt.Println("(could not open a browser automatically; use the URL above)")
	}

	select {
	case pd := <-pdCh:
		return &protonClient{ctx: ctx, pd: pd, folderSize: map[string]*ProtonEntry{}}, nil
	case err := <-srvErr:
		return nil, fmt.Errorf("local login page failed: %w", err)
	case <-time.After(15 * time.Minute):
		return nil, errors.New("timed out waiting for the Proton sign-in")
	}
}

// pageProtonLogin renders the login page, optionally with an error notice and
// the previously entered username.
func pageProtonLogin(errMsg, username string) string {
	twofaLabel := "(optional)"
	if strings.Contains(errMsg, "two-factor") {
		twofaLabel = "(required for this account)"
	}
	errHTML := ""
	if errMsg != "" {
		errHTML = `<p class="err">` + html.EscapeString(errMsg) + `</p>`
	}
	return fmt.Sprintf(protonLoginPage, html.EscapeString(username), twofaLabel, errHTML)
}

func (p *protonClient) logout() {
	if p.pd != nil {
		p.pd.Logout(p.ctx)
	}
}

// accountEmail returns the signed-in address for the welcome line (best
// effort; purely cosmetic).
func (p *protonClient) accountEmail() string {
	u, err := p.pd.About(p.ctx)
	if err != nil || u.Email == "" {
		return ""
	}
	return u.Email
}

// mkdir creates parentLinkID/name if it does not exist and returns its ID.
func (p *protonClient) mkdir(parentLinkID, name string) (string, error) {
	children, err := p.listDir(parentLinkID)
	if err != nil {
		return "", err
	}
	for _, c := range children {
		if c.IsFolder && c.Name == name {
			return c.Link.LinkID, nil
		}
	}
	return p.pd.CreateNewFolderByID(p.ctx, parentLinkID, name)
}

// listDir lists a folder once and caches the result for the dedupe check.
func (p *protonClient) listDir(linkID string) ([]*bridge.ProtonDirectoryData, error) {
	return p.pd.ListDirectory(p.ctx, linkID)
}

// hasEntry reports whether a folder already contains name with size sizeBytes
// (folders match on name alone). Used to skip files already uploaded by an
// earlier, interrupted run.
func (p *protonClient) hasEntry(folderLinkID, name string, isFolder bool, sizeBytes int64) bool {
	if _, ok := p.folderSize[folderLinkID]; !ok {
		p.folderSize[folderLinkID] = nil // mark as listed even if the call below fails
	}
	children, err := p.listDir(folderLinkID)
	if err != nil {
		return false
	}
	for _, c := range children {
		if c.Name != name {
			continue
		}
		if isFolder {
			return true
		}
		if c.Link != nil && c.Link.Size == sizeBytes {
			return true
		}
	}
	return false
}

// upload streams a file into parentLinkID under name.
func (p *protonClient) upload(parentLinkID, name string, modTime time.Time, r io.Reader) error {
	_, _, err := p.pd.UploadFileByReader(p.ctx, parentLinkID, name, modTime, r, 0)
	return err
}

func (p *protonClient) rootID() string {
	return p.pd.RootLink.LinkID
}
