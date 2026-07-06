//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

//go:embed editor.html
var editorHTML []byte

// startWeb runs the deck editor + layout API in the background.
func startWeb(port int) {
	mux := http.NewServeMux()

	// Web editor page (auto-opened in a browser on start).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(editorHTML)
	})

	// Trimmed layout the iPad fetches (labels + ids only, no action details).
	mux.HandleFunc("/deck", handleLayout)

	// Full config: GET to load into the editor, POST to save.
	mux.HandleFunc("/api/deck", handleAPIDeck)

	// Fire a button by id (used by the editor's Test buttons).
	mux.HandleFunc("/api/test", handleTest)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("web server: %v", err)
		}
	}()
}

func handleLayout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(layoutJSON())
}

func handleAPIDeck(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, getDeck())
	case http.MethodPost:
		var d Deck
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := setDeck(d); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.ID != "" {
		executeButton(body.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(v)
}

// openBrowser opens the given URL in the default browser (Windows).
func openBrowser(url string) {
	if err := exec.Command("cmd", "/c", "start", "", url).Start(); err != nil {
		log.Printf("could not open browser: %v", err)
	}
}
