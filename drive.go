package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Google Drive read-only client, hand-rolled on the REST API.
//
// Authentication is the standard installed-app OAuth flow: the user signs in
// in their browser, never in this app. The token lives only in memory for the
// duration of the run; nothing about the account or the session is written to
// disk, so each run starts with a browser sign-in.
// ---------------------------------------------------------------------------

const (
	googleAuthURL   = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL  = "https://oauth2.googleapis.com/token"
	driveAPIBase    = "https://www.googleapis.com/drive/v3"
	scopeDriveRO    = "https://www.googleapis.com/auth/drive.readonly"
	folderMime      = "application/vnd.google-apps.folder"
	shortcutMime    = "application/vnd.google-apps.shortcut"
	fieldSpec       = "nextPageToken, files(id, name, mimeType, size, modifiedTime, shortcutDetails(targetId))"
	uploadChunkSize = int64(8 * 1024 * 1024)
)

// exportMap converts Google-native documents to real files, exactly like
// Google Takeout and the earlier Python exporter do. They are not files on
// Drive and cannot be downloaded as-is.
var exportMap = map[string]struct{ Ext, MIME string }{
	"application/vnd.google-apps.document": {
		".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
	"application/vnd.google-apps.spreadsheet": {
		".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	"application/vnd.google-apps.presentation": {
		".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	"application/vnd.google-apps.drawing": {
		".png", "image/png"},
}

var skipMimes = map[string]bool{
	"application/vnd.google-apps.form":         true,
	"application/vnd.google-apps.script":       true,
	"application/vnd.google-apps.fusiontable":  true,
	"application/vnd.google-apps.map":          true,
}

type driveFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	Size         string `json:"size"`
	ModifiedTime string `json:"modifiedTime"`
	ShortcutDetails *struct {
		TargetID string `json:"targetId"`
	} `json:"shortcutDetails"`
}

func (f *driveFile) sizeBytes() int64 {
	n, _ := strconv.ParseInt(f.Size, 10, 64)
	return n
}

type fileListResp struct {
	NextPageToken string       `json:"nextPageToken"`
	Files         []*driveFile `json:"files"`
}

type driveClient struct {
	clientID     string
	clientSecret string
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time
	http         *http.Client
}

// credentials.json shapes Google hands out (installed / desktop app).
type oauthSecret struct {
	Installed *struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
	Web *struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"web"`
}

func newDriveClient(credentialsFile string) (*driveClient, error) {
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w (get it from Google Cloud Console → Credentials → OAuth client ID → Desktop app)", credentialsFile, err)
	}
	var sec oauthSecret
	if err := json.Unmarshal(raw, &sec); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", credentialsFile, err)
	}
	c := &driveClient{http: http.DefaultClient}
	switch {
	case sec.Installed != nil:
		c.clientID, c.clientSecret = sec.Installed.ClientID, sec.Installed.ClientSecret
	case sec.Web != nil:
		c.clientID, c.clientSecret = sec.Web.ClientID, sec.Web.ClientSecret
	default:
		return nil, fmt.Errorf("%s has neither an \"installed\" nor a \"web\" client (choose the Desktop app type in the Console)", credentialsFile)
	}
	return c, nil
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

// authorize signs the user in through the browser; the token stays in memory.
func (c *driveClient) authorize() error {
	// The loopback redirect is served by a throwaway local listener; Google
	// redirects there with ?code=..., and the browser tab is closed by JS.
	codeCh := make(chan string, 1)
	srvErr := make(chan error, 1)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := l.Addr().(*net.TCPAddr).Port
	redirect := fmt.Sprintf("http://127.0.0.1:%d/", port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Fprint(w, "<body><script>window.close()</script>Authorization received; you can close this tab.</body>")
			return
		}
		fmt.Fprint(w, "<body><script>window.close()</script>Sign-in complete; you can close this tab and return to the terminal.</body>")
		codeCh <- code
	})
	srv := &http.Server{Handler: mux}
	go func() { srvErr <- srv.Serve(l) }()
	defer srv.Close()

	params := url.Values{}
	params.Set("client_id", c.clientID)
	params.Set("redirect_uri", redirect)
	params.Set("response_type", "code")
	params.Set("scope", scopeDriveRO)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	authURL := googleAuthURL + "?" + params.Encode()

	fmt.Println("\nOpening your browser to sign in to Google Drive (read-only access).")
	fmt.Println("If it does not open, paste this URL into a browser:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	if err := openBrowser(authURL); err != nil {
		fmt.Println("(could not open a browser automatically; use the URL above)")
	}

	select {
	case code := <-codeCh:
		srv.Close()
		return c.exchange(code, redirect)
	case err := <-srvErr:
		return fmt.Errorf("local auth listener failed: %w", err)
	case <-time.After(10 * time.Minute):
		return fmt.Errorf("timed out waiting for the Google sign-in")
	}
}

func (c *driveClient) exchange(code, redirect string) error {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("redirect_uri", redirect)
	form.Set("grant_type", "authorization_code")
	resp, err := http.PostForm(googleTokenURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var tr tokenResp
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("token exchange: HTTP %d", resp.StatusCode)
	}
	if tr.Error != "" {
		return fmt.Errorf("token exchange: %s", tr.Error)
	}
	c.accessToken = tr.AccessToken
	c.refreshToken = tr.RefreshToken
	c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return nil
}

func (c *driveClient) refresh() error {
	if c.refreshToken == "" {
		return fmt.Errorf("no refresh token; sign in again")
	}
	form := url.Values{}
	form.Set("refresh_token", c.refreshToken)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "refresh_token")
	resp, err := http.PostForm(googleTokenURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var tr tokenResp
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return fmt.Errorf("token refresh failed (HTTP %d)", resp.StatusCode)
	}
	c.accessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		c.refreshToken = tr.RefreshToken
	}
	c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return nil
}

// login() gets a working access token through a fresh browser sign-in.
// Nothing is cached between runs: the token lives only in this process.
func (c *driveClient) login() error {
	if c.accessToken == "" {
		return c.authorize()
	}
	if time.Now().After(c.tokenExpiry.Add(-2 * time.Minute)) {
		if err := c.refresh(); err != nil {
			// A stale cached token without a refresh token: start over.
			return c.authorize()
		}
	}
	return nil
}

// api performs an authenticated Drive API call, refreshing once on 401.
func (c *driveClient) api(method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	call := func() (*http.Response, error) {
		u := driveAPIBase + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		req, err := http.NewRequest(method, u, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return c.http.Do(req)
	}
	resp, err := retry(call)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		resp.Body.Close()
		if err := c.refresh(); err != nil {
			return nil, err
		}
		resp, err = call()
	}
	return resp, err
}

func closeAndError(resp *http.Response, what string) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return fmt.Errorf("%s: HTTP %d: %s", what, resp.StatusCode, strings.TrimSpace(string(b)))
}

func (c *driveClient) getJSON(what, path string, query url.Values, out interface{}) error {
	resp, err := c.api("GET", path, query, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return closeAndError(resp, what)
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *driveClient) getRootID() (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.getJSON("getting Drive root", "/files/root", url.Values{"fields": {"id"}}, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *driveClient) getMeta(fileID string) (*driveFile, error) {
	q := url.Values{"fields": {"id, name"}, "supportsAllDrives": {"true"}}
	var f driveFile
	if err := c.getJSON("getting folder name", "/files/"+url.PathEscape(fileID), q, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// listChildren yields every non-trashed child of folderID, across pages.
// Shared items and Shared Drive content are included via supportsAllDrives.
func (c *driveClient) listChildren(folderID string) ([]*driveFile, error) {
	var out []*driveFile
	pageToken := ""
	for {
		q := url.Values{
			"q":                         {fmt.Sprintf("'%s' in parents and trashed = false", folderID)},
			"fields":                    {fieldSpec},
			"pageSize":                  {"1000"},
			"supportsAllDrives":         {"true"},
			"includeItemsFromAllDrives": {"true"},
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		var resp fileListResp
		if err := c.getJSON("listing folder", "/files", q, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Files...)
		if resp.NextPageToken == "" {
			return out, nil
		}
		pageToken = resp.NextPageToken
	}
}

type sharedDrive struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *driveClient) listSharedDrives() ([]*sharedDrive, error) {
	var out []*sharedDrive
	pageToken := ""
	for {
		q := url.Values{"pageSize": {"100"}}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		var resp struct {
			NextPageToken string         `json:"nextPageToken"`
			Drives        []*sharedDrive `json:"drives"`
		}
		if err := c.getJSON("listing shared drives", "/drives", q, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Drives...)
		if resp.NextPageToken == "" {
			return out, nil
		}
		pageToken = resp.NextPageToken
	}
}

// download streams a file (or an export of a Google-native document) into w.
func (c *driveClient) download(fileID, exportMime string, w io.Writer) error {
	path := "/files/" + url.PathEscape(fileID)
	if exportMime != "" {
		q := url.Values{"mimeType": {exportMime}, "supportsAllDrives": {"true"}}
		path += "/export?" + q.Encode()
	} else {
		path += "?alt=media&supportsAllDrives=true"
	}
	resp, err := c.api("GET", path, nil, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return closeAndError(resp, "downloading")
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}

func openBrowser(u string) error {
	var cmd *exec.Cmd
	switch {
	case commandExists("xdg-open"):
		cmd = exec.Command("xdg-open", u)
	case commandExists("open"):
		cmd = exec.Command("open", u)
	case commandExists("cmd"):
		cmd = exec.Command("cmd", "/c", "start", "", u)
	default:
		return fmt.Errorf("no browser launcher found")
	}
	return cmd.Start()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func parseRFC3339(ts string) time.Time {
	t, _ := time.Parse(time.RFC3339, ts)
	return t
}
