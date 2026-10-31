package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

// inventory.go bridges the site's Assess step to the Python inventory:
// the browser asks /api/inventory, the Go server (already holding the
// read-only Google token from the browser sign-in) runs inventory.py in web
// mode and returns the aggregate rows as JSON. Only aggregates travel back;
// no names, no content.

var inventoryRunning atomic.Bool

type invRow struct {
	Service string  `json:"service"`
	Metric  string  `json:"metric"`
	Value   float64 `json:"value,string"`
	Unit    string  `json:"unit"`
	Detail  string  `json:"detail"`
}

func handleInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `{"error":"method"}`)
		return
	}
	if !inventoryRunning.CompareAndSwap(false, true) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error":"busy","message":"An inventory run is already in progress."}`)
		return
	}
	defer inventoryRunning.Store(false)

	ws.mu.Lock()
	gc := ws.gc
	ws.mu.Unlock()
	if gc == nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"signin","message":"Sign in with Google first.","url":"/migrate"}`)
		return
	}

	invDir, err := filepath.Abs("inventory")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"dir","message":%q}`, err.Error())
		return
	}
	out := filepath.Join(invDir, "reports")
	os.MkdirAll(out, 0o700)
	// Fresh aggregates for each run (stale run_ids would double-count).
	os.Remove(filepath.Join(out, "agregados.csv"))
	os.Remove(filepath.Join(out, "reporte.md"))
	os.Remove(filepath.Join(invDir, "data", "inventory.db"))

	env := append(os.Environ(),
		"INVENTORY_ACCESS_TOKEN="+gc.accessToken,
	)
	run := func(args ...string) error {
		cmd := exec.Command("python3", append([]string{"inventory.py"}, args...)...)
		cmd.Dir = invDir
		cmd.Env = env
		outp, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, tail(string(outp), 300))
		}
		return nil
	}
	if err := run("run"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"run","message":%s}`, jsonString(err.Error()))
		return
	}
	if err := run("report"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"report","message":%s}`, jsonString(err.Error()))
		return
	}

	f, err := os.Open(filepath.Join(out, "agregados.csv"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"read","message":%s}`, jsonString(err.Error()))
		return
	}
	defer f.Close()
	rows, err := parseAgregados(f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"parse","message":%s}`, jsonString(err.Error()))
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"rows": rows})
}

func parseAgregados(f *os.File) ([]invRow, error) {
	cr := csv.NewReader(f)
	cr.FieldsPerRecord = -1
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	var rows []invRow
	for i, rec := range recs {
		if i == 0 || len(rec) < 5 {
			continue
		}
		v, _ := strconv.ParseFloat(strings.TrimSpace(rec[3]), 64)
		rows = append(rows, invRow{Service: rec[1], Metric: rec[2], Value: v, Unit: rec[4], Detail: strings.Join(rec[5:], ",")})
	}
	return rows, nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
