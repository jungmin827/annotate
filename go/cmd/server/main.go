package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"annotate/internal/handler"
	"annotate/internal/store"
)

func main() {
	_, src, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(src), "..", "..", "..")

	dbPath := filepath.Join(root, "data", "annotate.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.SQL.Close()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "warning: ANTHROPIC_API_KEY not set; analysis endpoints will fail")
	}

	h := handler.New(db, apiKey)
	addr := ":8080"
	fmt.Printf("Stock-Ops server listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, h))
}
