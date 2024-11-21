package web

import (
	"fmt"
	"net/http"

	"github.com/xperimental/logging-roundtrip/internal/sink"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

func livenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "I'm ok.")
	}
}

func countHandler(store *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count := store.Count()
		fmt.Fprintln(w, count)
	}
}

func disconnectHandler(sink sink.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sink.Disconnect()
		fmt.Fprintln(w, "done.")
	}
}
