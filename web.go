package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Web mode: the same engine as the CLI, behind a small wizard so the tool can
// run on a server (docker / PaaS) with nothing but a browser.
//
// The security model is unchanged: the Google token and the Proton session
// exist only in memory for the duration of the run; nothing sensitive is
// written to disk. The only files kept under DATA_DIR are the resume manifest
// (Google file IDs, no credentials) and the Google OAuth client id/secret the
// operator enters once.
// ---------------------------------------------------------------------------

const appVersionWeb = "external-drive-gdrive2proton@1.0.0-beta+gdrive2proton"

// webScopes is the read-only union the web edition asks consent for: the
// Drive scope the migration needs plus the read-only scopes the Python
// inventory uses for the Assess step. Nothing here allows writing or
// deleting anything in the Google account.
const webScopes = "https://www.googleapis.com/auth/drive.readonly " +
	"https://www.googleapis.com/auth/gmail.readonly " +
	"https://www.googleapis.com/auth/calendar.readonly " +
	"https://www.googleapis.com/auth/contacts.readonly " +
	"https://www.googleapis.com/auth/photoslibrary.readonly " +
	"https://www.googleapis.com/auth/tasks.readonly"

type webState struct {
	mu          sync.Mutex
	gc          *driveClient
	pc          *protonClient
	protonEmail string
	protonUsed  int64
	protonMax   int64
	oauthState  string

	running bool
	done    bool
	runErr  string
	st      *stats
	topName string
	log     []string
}

// addLog appends to the web log; the caller must hold s.mu.
func (s *webState) addLog(msg string) {
	s.log = append(s.log, time.Now().Format("15:04:05")+"  "+msg)
	if len(s.log) > 200 {
		s.log = s.log[len(s.log)-200:]
	}
}

var ws = &webState{st: &stats{}}

func dataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	return "/data"
}

func googleClientFile() string { return filepath.Join(dataDir(), "google-client.json") }

func loadGoogleClientIDSecret() (id, secret string, ok bool) {
	if id = os.Getenv("GOOGLE_CLIENT_ID"); id != "" {
		secret = os.Getenv("GOOGLE_CLIENT_SECRET")
		return id, secret, true
	}
	var rec struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if b, err := os.ReadFile(googleClientFile()); err == nil {
		if json.Unmarshal(b, &rec) == nil && rec.ClientID != "" {
			return rec.ClientID, rec.ClientSecret, true
		}
	}
	return "", "", false
}

func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Pages
// ---------------------------------------------------------------------------

func pageHTML(title, body string) string {
	return `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
body{font-family:system-ui,sans-serif;max-width:640px;margin:0 auto;padding:2rem 1rem;color:#1a1a1a;background:#fafafa}
h1{font-size:1.4rem} h2{font-size:1.05rem;margin-top:2rem}
.card{background:#fff;border:1px solid #e4e4e4;border-radius:10px;padding:1rem 1.25rem;margin:.75rem 0}
.ok{color:#0a7d33;font-weight:600}.pending{color:#9a9a9a}
button,input[type=text],input[type=password]{font:inherit;padding:.5rem .75rem;border:1px solid #ccc;border-radius:8px}
button{background:#6d4aff;color:#fff;border:none;cursor:pointer;padding:.55rem 1.1rem}
a button{color:#fff;text-decoration:none}
small{color:#666}code{background:#eee;padding:.1rem .3rem;border-radius:4px;font-size:.85em}
#log{font-family:ui-monospace,monospace;font-size:.78rem;white-space:pre-wrap;background:#141418;color:#d5d5d5;border-radius:8px;padding:.8rem;max-height:320px;overflow-y:auto}
.err{color:#b3261e}
</style></head><body>` + body + `</body></html>`
}

func (s *webState) buildHomePage() string {
	s.mu.Lock()
	google := s.gc != nil
	proton := s.pc != nil
	running := s.running
	topName := s.topName
	s.mu.Unlock()

	_, _, hasClient := loadGoogleClientIDSecret()

	step := func(ok bool, num, label string) string {
		mark, cls := "&#9679;", "ok"
		if !ok {
			mark, cls = "&#9675;", "pending"
		}
		return `<span class="` + cls + `">` + mark + `</span> <b>` + num + `. ` + label + `</b>`
	}

	var b strings.Builder
	b.WriteString(`<h1>gdrive2proton <small>&mdash; Google Drive &rarr; Proton Drive</small></h1>
<p><a href="/">Back to the migration kit</a></p>`)

	if !hasClient {
		b.WriteString(`<div class="card"><b>One-time setup:</b> paste a Google OAuth <i>Web application</i> client.
Create one at <code>console.cloud.google.com</code> (APIs &amp; Services &rarr; Credentials), enable the
Google Drive API, and add <code>https://` + html.EscapeString(hostname()) + `/migrate/oauth/google/callback</code>
as an authorized redirect URI. The secret is stored under <code>` + html.EscapeString(dataDir()) + `</code> only.
<form method="post" action="/migrate/setup">
<input type="text" name="client_id" placeholder="Client ID" required style="width:100%;margin:.3rem 0">
<input type="password" name="client_secret" placeholder="Client secret" required style="width:100%;margin:.3rem 0">
<button>Save</button></form></div>`)
	}

	b.WriteString(`<div class="card">` + step(google, "1", "Sign in to Google Drive (read-only)"))
	if google {
		b.WriteString(` <span class="ok">&#10003;</span>`)
	} else if hasClient {
		b.WriteString(` <a href="/migrate/oauth/google/start"><button>Sign in with Google</button></a>`)
	}
	b.WriteString(`<br>` + step(proton, "2", "Sign in to Proton Drive"))
	if proton {
		b.WriteString(` <span class="ok">&#10003;</span>`)
	} else if google {
		b.WriteString(` <a href="/migrate/proton"><button>Sign in to Proton</button></a>`)
	}
	b.WriteString(`<br>` + step(false, "3", "Copy everything") + `</div>`)

	if google && proton {
		b.WriteString(`<div class="card"><form method="post" action="/migrate/start">
<label>Name of the folder created in Proton Drive<br><input type="text" name="dest" value="` +
			html.EscapeString(topName) + `" placeholder="Google Drive (default)"></label><br><br>
<label><input type="checkbox" name="shared" value="1"> also copy every Shared Drive I can access</label><br><br>
<label>Copy only this Drive folder ID (optional, default: all of My Drive)<br>
<input type="text" name="folder" placeholder="leave empty for everything"></label><br><br>
<button type="submit" ` + map[bool]string{true: "disabled", false: ""}[running] + `>Start copying</button>
</form></div>`)
	}

	b.WriteString(`<h2>Progress</h2><div id="log">loading&hellip;</div>
<script>
async function tick(){
  try{
    const r=await fetch('/migrate/status',{cache:'no-store'});const j=await r.json();
    let t='';
    if(j.running||j.done){
      t+='uploaded: '+j.uploaded+'   already there: '+j.skipped+'   not copyable: '+j.notCopyable+'\n';
      if(j.lastFile)t+='current: '+j.lastFile+'\n';
    }
    if(j.error)t+='last error: '+j.error+'\n';
    t+=(j.log||[]).join('\n');
    document.getElementById('log').textContent=t||'idle';
  }catch(e){}
  setTimeout(tick,2000);
}
tick();
</script>`)
	b.WriteString(`<p><small>Read-only Google scope; uploads go through Proton's end-to-end encryption.
Nothing sensitive is stored: tokens and the Proton session live in memory only.
The resume manifest (Google file IDs, no credentials) is kept under ` + html.EscapeString(dataDir()) + `.</small></p>`)
	return pageHTML("gdrive2proton", b.String())
}

func hostname() string {
	if h := os.Getenv("BASE_URL"); h != "" {
		return strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	}
	return ""
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	id := strings.TrimSpace(r.PostFormValue("client_id"))
	secret := strings.TrimSpace(r.PostFormValue("client_secret"))
	if id == "" || secret == "" {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	os.MkdirAll(dataDir(), 0o700)
	b, _ := json.Marshal(map[string]string{"client_id": id, "client_secret": secret})
	os.WriteFile(googleClientFile(), b, 0o600)
	http.Redirect(w, r, "/migrate", http.StatusSeeOther)
}

func handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	id, _, ok := loadGoogleClientIDSecret()
	if !ok {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	state := randomToken()
	ws.mu.Lock()
	ws.oauthState = state
	ws.mu.Unlock()

	q := url.Values{
		"client_id":     {id},
		"redirect_uri":  {"https://" + r.Host + "/migrate/oauth/google/callback"},
		"response_type": {"code"},
		"scope":         {webScopes},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}
	http.Redirect(w, r, googleAuthURL+"?"+q.Encode(), http.StatusSeeOther)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	ws.mu.Lock()
	expect := ws.oauthState
	ws.oauthState = ""
	ws.mu.Unlock()
	if expect == "" || r.URL.Query().Get("state") != expect {
		fmt.Fprint(w, pageHTML("gdrive2proton", `<div class="card err">Authorization state mismatch; start again.</div><a href="/migrate">Back</a>`))
		return
	}
	id, secret, ok := loadGoogleClientIDSecret()
	if !ok {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	gc := &driveClient{clientID: id, clientSecret: secret, http: http.DefaultClient}
	if err := gc.exchange(r.URL.Query().Get("code"), "https://"+r.Host+"/migrate/oauth/google/callback"); err != nil {
		fmt.Fprint(w, pageHTML("gdrive2proton", `<div class="card err">Google sign-in failed: `+html.EscapeString(err.Error())+`</div><a href="/migrate">Back</a>`))
		return
	}
	ws.mu.Lock()
	ws.gc = gc
	ws.mu.Unlock()
	http.Redirect(w, r, "/migrate", http.StatusSeeOther)
}

func handleProtonPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleProtonSubmit(w, r)
		return
	}
	ws.mu.Lock()
	signed := ws.pc != nil
	ws.mu.Unlock()
	if signed {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	fmt.Fprint(w, pageHTML("Sign in to Proton Drive", pageProtonLogin("", "")))
}

func handleProtonSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	twofa := strings.TrimSpace(r.PostFormValue("twofa"))

	pd, err := protonSignIn(context.Background(), appVersionWeb, username, password, twofa)
	if err != nil {
		msg := "Sign-in failed. Check the username and password."
		switch {
		case strings.Contains(strings.ToLower(err.Error()), "2fa"):
			msg = "This account has two-factor authentication: enter the 6-digit code from your authenticator app."
		case strings.Contains(strings.ToLower(err.Error()), "password"):
			msg = "Sign-in failed: wrong username or password."
		}
		fmt.Fprint(w, pageHTML("Sign in to Proton Drive", pageProtonLogin(msg, html.EscapeString(username))))
		return
	}
	pc := &protonClient{ctx: context.Background(), pd: pd, folderSize: map[string]*ProtonEntry{}}
	email := pc.accountEmail()
	var used, max int64
	if about, aerr := pd.About(pc.ctx); aerr == nil && about != nil {
		used, max = about.UsedSpace, about.MaxSpace
	}
	ws.mu.Lock()
	ws.pc = pc
	ws.protonEmail = email
	ws.protonUsed, ws.protonMax = used, max
	ws.addLog("signed in to Proton Drive as " + email)
	ws.mu.Unlock()
	// The password is not kept: it lived only in this request.
	http.Redirect(w, r, "/migrate", http.StatusSeeOther)
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	ws.mu.Lock()
	gc, pc, running := ws.gc, ws.pc, ws.running
	ws.mu.Unlock()
	if gc == nil || pc == nil || running {
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}

	dest := strings.TrimSpace(r.PostFormValue("dest"))
	folder := strings.TrimSpace(r.PostFormValue("folder"))
	shared := r.PostFormValue("shared") == "1"

	rootID := folder
	if rootID == "" {
		var err error
		rootID, err = gc.getRootID()
		if err != nil {
			ws.mu.Lock(); ws.runErr = err.Error(); ws.mu.Unlock()
			http.Redirect(w, r, "/migrate", http.StatusSeeOther)
			return
		}
	}
	meta, err := gc.getMeta(rootID)
	if err != nil {
		ws.mu.Lock(); ws.runErr = err.Error(); ws.mu.Unlock()
		http.Redirect(w, r, "/migrate", http.StatusSeeOther)
		return
	}
	topName := dest
	if topName == "" {
		topName = sanitize(meta.Name)
	}

	ws.mu.Lock()
	ws.running = true
	ws.done = false
	ws.runErr = ""
	ws.st = &stats{}
	ws.log = nil
	ws.topName = topName
	ws.addLog("starting copy of " + meta.Name)
	st := ws.st
	ws.mu.Unlock()

	go func() {
		targetRoot, err := pc.mkdir(pc.rootID(), topName)
		if err == nil {
			err = copyTree(gc, pc, rootID, targetRoot, "Proton Drive / "+topName, map[string]bool{}, st, false)
		}
		if err != nil {
			st.addError(err.Error())
		}
		if shared {
			drives, derr := gc.listSharedDrives()
			if derr != nil {
				st.addError(fmt.Sprintf("listing shared drives: %v", derr))
			}
			for _, d := range drives {
				name := sanitize(d.Name)
				target, merr := pc.mkdir(pc.rootID(), name)
				if merr != nil {
					st.addError(fmt.Sprintf("creating folder %s: %v", name, merr))
					continue
				}
				if terr := copyTree(gc, pc, d.ID, target, "Proton Drive / "+name, map[string]bool{}, st, false); terr != nil {
					st.addError(terr.Error())
				}
			}
		}
		pc.logout()
		ws.mu.Lock()
		ws.running = false
		ws.done = true
		ws.pc = nil
		if len(st.errors) > 0 {
			ws.runErr = strings.Join(st.errors, "; ")
		}
		ws.addLog(fmt.Sprintf("finished: %d uploaded, %d skipped, %d not copyable", st.uploaded, st.skipped, len(st.errors)))
		ws.mu.Unlock()
		if manifestFile != nil {
			manifestFile.Close()
			manifestFile = nil
		}
	}()

	http.Redirect(w, r, "/migrate", http.StatusSeeOther)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	ws.mu.Lock()
	up, skip, notCopyable, current := ws.st.snapshot()
	st := map[string]any{
		"google":      ws.gc != nil,
		"proton":      ws.pc != nil,
		"running":     ws.running,
		"done":        ws.done,
		"uploaded":    up,
		"skipped":     skip,
		"notCopyable": notCopyable,
		"lastFile":    current,
		"error":       ws.runErr,
		"log":         ws.log,
		"protonEmail": ws.protonEmail,
	}
	if ws.protonMax > 0 {
		st["protonUsedPct"] = 100 * float64(ws.protonUsed) / float64(ws.protonMax)
	}
	ws.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st)
}

func runWeb() {
	port := os.Getenv("PORT")
	if _, err := strconv.Atoi(port); err != nil {
		port = "3000"
	}
	// Keep the resume manifest under DATA_DIR (the only persistent volume on
	// most PaaS deploys), not next to the binary.
	manifestName = filepath.Join(dataDir(), ".gdrive2proton_manifest.jsonl")
	os.MkdirAll(dataDir(), 0o700)

	mux := http.NewServeMux()

	// The migration kit site (index.html, assistant.html) is served at /.
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("site"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/migrate") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join("site", filepath.Clean("/"+r.URL.Path)))
	})

	// The Drive -> Proton wizard lives under /migrate.
	mux.HandleFunc("/migrate", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ws.buildHomePage())
	})
	mux.HandleFunc("/migrate/setup", handleSetup)
	mux.HandleFunc("/migrate/oauth/google/start", handleGoogleStart)
	mux.HandleFunc("/migrate/oauth/google/callback", handleGoogleCallback)
	mux.HandleFunc("/migrate/proton", handleProtonPage)
	mux.HandleFunc("/migrate/start", handleStart)
	mux.HandleFunc("/migrate/status", handleStatus)

	// Assess step: run the Python inventory with the read-only Google token.
	mux.HandleFunc("/api/inventory", handleInventory)

	// A panic in one handler must never take the whole app down: recover,
	// log, and answer 500 so the platform proxy shows an error page instead
	// of a Bad Gateway for every later request.
	logged := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("panic in %s %s: %v\n", r.Method, r.URL.Path, rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		mux.ServeHTTP(w, r)
	})

	addr := "0.0.0.0:" + port
	fmt.Println("gdrive2proton web mode, listening on", addr)
	if err := http.ListenAndServe(addr, logged); err != nil {
		fatal(err)
	}
}
