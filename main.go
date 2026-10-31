package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// gdrive2proton copies a Google Drive account into Proton Drive, preserving
// the folder hierarchy, as a single self-contained binary. Both sign-ins
// happen in the browser; the tool holds a read-only Google token and uploads
// through Proton's end-to-end encryption (Proton-API-Bridge). No credentials,
// tokens or keys are written to disk: everything sensitive lives in memory for
// the duration of the run and is gone when it ends.

var manifestName = ".gdrive2proton_manifest.jsonl"

type stats struct {
	mu          sync.Mutex
	uploaded    int
	skipped     int
	errors      []string
	uploading   int64 // bytes done for the current file
	currentFile string
}

// markUploaded / markSkipped: counters are read live by the web dashboard.
func (s *stats) markUploaded() { s.mu.Lock(); s.uploaded++; s.mu.Unlock() }
func (s *stats) markSkipped()  { s.mu.Lock(); s.skipped++; s.mu.Unlock() }
func (s *stats) snapshot() (up, skip, notCopyable int, current string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.uploaded, s.skipped, len(s.errors), s.currentFile
}
func (s *stats) addError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, msg)
}

func main() {
	// Web mode: when the platform gives us a port (or WEB=1 is set), serve the
	// browser wizard instead of running the terminal CLI.
	if os.Getenv("PORT") != "" || os.Getenv("WEB") == "1" {
		runWeb()
		return
	}

	var (
		gclient   = flag.String("gclient", "credentials.json", "Google OAuth client JSON (Google Cloud Console, Desktop app type)")
		pappver   = flag.String("proton-appversion", "external-drive-gdrive2proton@1.0.0-beta+gdrive2proton",
			"app identifier sent to Proton (change only if Proton rejects it)")
		folderID  = flag.String("folder", "", "copy only the subtree of this Drive folder ID (default: all of My Drive)")
		sharedDrv = flag.Bool("shared-drives", false, "also copy every Shared Drive you can access")
		topFolder = flag.String("dest-folder", "", "name of the folder created in Proton Drive (default: the Drive folder's name)")
		dryRun    = flag.Bool("dry-run", false, "list what would be copied without uploading anything")
		showVer   = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("gdrive2proton 1.0.0")
		return
	}

	fmt.Println("gdrive2proton — copy Google Drive to Proton Drive")
	fmt.Println("  Both sign-ins open in your browser. Nothing sensitive is saved to disk.")
	fmt.Println()

	ctx := context.Background()

	// --- Google sign-in --------------------------------------------------
	gc, err := newDriveClient(*gclient)
	if err != nil {
		fatal(err)
	}
	if err := gc.login(); err != nil {
		fatal(err)
	}
	fmt.Println("Google Drive: signed in.")

	// --- Proton sign-in --------------------------------------------------
	pc, err := newProtonClient(ctx, *pappver)
	if err != nil {
		fatal(err)
	}
	defer pc.logout()
	fmt.Println("Proton Drive: signed in.")
	if email := pc.accountEmail(); email != "" {
		if u, err := pc.pd.About(ctx); err == nil {
			fmt.Printf("  account: %s — %s used of %s\n", email, humanSize(u.UsedSpace), humanSize(u.MaxSpace))
		}
	}
	fmt.Println()

	// The resume manifest holds only Google file IDs and names, so an
	// interrupted run can skip what was already uploaded. It contains no
	// credentials; delete it when the migration is finished.
	done := loadManifest(manifestName)
	st := &stats{}
	started := time.Now()

	// The copy lands inside a folder named after the source, so re-runs and
	// additional sources (shared drives) nest cleanly.
	rootID := *folderID
	if rootID == "" {
		rootID, err = gc.getRootID()
		if err != nil {
			fatal(err)
		}
	}
	rootMeta, err := gc.getMeta(rootID)
	if err != nil {
		fatal(err)
	}
	topName := *topFolder
	if topName == "" {
		topName = sanitize(rootMeta.Name)
	}

	targetRoot, err := pc.mkdir(pc.rootID(), topName)
	if err != nil {
		fatal(err)
	}
	topLabel := "Proton Drive / " + topName
	if err := copyTree(gc, pc, rootID, targetRoot, topLabel, done, st, *dryRun); err != nil {
		st.addError(err.Error())
	}

	if *sharedDrv {
		drives, err := gc.listSharedDrives()
		if err != nil {
			st.addError(fmt.Sprintf("listing shared drives: %v", err))
		}
		for _, d := range drives {
			name := sanitize(d.Name)
			target, err := pc.mkdir(pc.rootID(), name)
			if err != nil {
				st.addError(fmt.Sprintf("creating folder %s: %v", name, err))
				continue
			}
			if err := copyTree(gc, pc, d.ID, target, "Proton Drive / "+name, done, st, *dryRun); err != nil {
				st.addError(err.Error())
			}
		}
	}

	minutes := time.Since(started).Minutes()
	fmt.Println()
	fmt.Printf("Done in %.1f min: %d uploaded, %d already there (skipped), %d not copyable.\n",
		minutes, st.uploaded, st.skipped, len(st.errors))
	if len(st.errors) > 0 {
		fmt.Println("\nErrors (re-run the same command to retry):")
		for _, e := range st.errors {
			fmt.Printf("  - %s\n", e)
		}
		os.Exit(1)
	}
}

// copyTree mirrors a Drive folder into a Proton folder, depth-first.
// label is the display path used in progress output.
func copyTree(gc *driveClient, pc *protonClient, driveFolderID, protonFolderID, label string, done map[string]bool, st *stats, dryRun bool) error {
	children, err := gc.listChildren(driveFolderID)
	if err != nil {
		return fmt.Errorf("listing Drive folder %s: %w", driveFolderID, err)
	}

	var folders, files []*driveFile
	for _, c := range children {
		switch {
		case c.MimeType == folderMime:
			folders = append(folders, c)
		case c.MimeType == shortcutMime:
			note := fmt.Sprintf("shortcut -> %s", c.ShortcutDetails.TargetID)
			fmt.Printf("  [not copyable] %s (%s)\n", c.Name, note)
			st.addError(fmt.Sprintf("%s (%s)", c.Name, note))
		case skipMimes[c.MimeType]:
			fmt.Printf("  [not copyable] %s (%s: Drive cannot export it as a file)\n", c.Name, c.MimeType)
			st.addError(fmt.Sprintf("%s (%s)", c.Name, c.MimeType))
		default:
			files = append(files, c)
		}
	}

	fmt.Printf("%s: %d folders, %d files\n", label, len(folders), len(files))

	for _, f := range files {
		if done[f.ID] {
			st.markSkipped()
			continue
		}
		if err := copyFile(gc, pc, f, protonFolderID, st, dryRun); err != nil {
			st.addError(fmt.Sprintf("%s: %v", f.Name, err))
		}
	}

	for _, sub := range folders {
		name := sanitize(sub.Name)
		var subID string
		if dryRun {
			subID = "dry-run"
			fmt.Printf("  [dry-run] folder %s/\n", name)
		} else {
			subID, err = pc.mkdir(protonFolderID, name)
			if err != nil {
				st.addError(fmt.Sprintf("creating folder %s: %v", name, err))
				continue
			}
		}
		if err := copyTree(gc, pc, sub.ID, subID, label+"/"+name, done, st, dryRun); err != nil {
			st.addError(err.Error())
		}
	}
	return nil
}

// copyFile downloads from Google to a temp file and uploads to Proton,
// converting Google-native documents to real files on the way.
func copyFile(gc *driveClient, pc *protonClient, f *driveFile, protonFolderID string, st *stats, dryRun bool) error {
	name := sanitize(f.Name)
	exportExt := ""
	exportMime := ""
	if e, ok := exportMap[f.MimeType]; ok {
		exportExt, exportMime = e.Ext, e.MIME
		name = strings.TrimSuffix(name, filepath.Ext(name)) + exportExt
	}
	size := f.sizeBytes()
	st.mu.Lock()
	st.currentFile = name
	st.mu.Unlock()
	if dryRun {
		fmt.Printf("  [dry-run] %s (%s)\n", name, humanSize(size))
		st.markUploaded()
		return nil
	}

	if pc.hasEntry(protonFolderID, name, false, size) {
		fmt.Printf("  [already there] %s\n", name)
		recordDone(f.ID, "")
		st.markSkipped()
		return nil
	}

	modTime := parseRFC3339(f.ModifiedTime)
	if modTime.IsZero() {
		modTime = time.Now()
	}

	tmp, err := os.CreateTemp("", "gdrive2proton-*.part")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	fmt.Printf("  ↓ %s (%s) ... ", name, humanSize(size))
	if err := gc.download(f.ID, exportMime, tmp); err != nil {
		tmp.Close()
		fmt.Printf("download failed: %v\n", err)
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		return err
	}
	st.uploading = 0
	if err := pc.upload(protonFolderID, name, modTime, tmp); err != nil {
		tmp.Close()
		fmt.Printf("upload failed: %v\n", err)
		return err
	}
	tmp.Close()
	fmt.Println("✓")
	st.markUploaded()
	recordDone(f.ID, name)
	return nil
}

// --- resume manifest ---------------------------------------------------------

func loadManifest(path string) map[string]bool {
	done := map[string]bool{}
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var rec struct {
				ID string `json:"id"`
			}
			if json.Unmarshal([]byte(line), &rec) == nil && rec.ID != "" {
				done[rec.ID] = true
			}
		}
	}
	return done
}

var manifestFile *os.File

func recordDone(id, name string) {
	if manifestFile == nil {
		f, err := os.OpenFile(manifestName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		manifestFile = f
	}
	b, _ := json.Marshal(map[string]string{"id": id, "name": name})
	manifestFile.Write(append(b, '\n'))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
