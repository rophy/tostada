//go:build coverage

package main

import (
	"archive/tar"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/coverage"
)

func init() {
	registerCoverageHandler = func(mux *http.ServeMux) {
		mux.HandleFunc("/debug/coverage/flush", handleCoverageFlush)
		log.Println("Coverage flush endpoint enabled at /debug/coverage/flush")
	}
}

func handleCoverageFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	dir, err := os.MkdirTemp("", "covdata-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(dir)

	if err := coverage.WriteMetaDir(dir); err != nil {
		http.Error(w, "write meta: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := coverage.WriteCountersDir(dir); err != nil {
		http.Error(w, "write counters: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-tar")
	tw := tar.NewWriter(w)
	defer tw.Close()

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			continue
		}
		hdr.Name = e.Name()
		if err := tw.WriteHeader(hdr); err != nil {
			return
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		io.Copy(tw, f)
		f.Close()
	}
}
