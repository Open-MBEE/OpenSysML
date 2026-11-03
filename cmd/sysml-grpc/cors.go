package main

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	corsHeaders       = "Authorization, Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, Grpc-Timeout, X-Grpc-Web, X-User-Agent, Accept, Accept-Encoding, Connect-Content-Encoding, Connect-Accept-Encoding, Grpc-Encoding, Grpc-Accept-Encoding"
	corsExposeHeaders = "WWW-Authenticate, Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin, Connect-Content-Encoding, Connect-Accept-Encoding, Grpc-Encoding, Grpc-Accept-Encoding"
)

func parseCORSOrigins(value string) (map[string]struct{}, error) {
	origins := map[string]struct{}{}
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, fmt.Errorf("-cors-allowed-origins does not allow the wildcard origin")
		}
		origins[origin] = struct{}{}
	}
	return origins, nil
}

func corsMiddleware(next http.Handler, allowed map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		_, ok := allowed[origin]
		w.Header().Add("Vary", "Origin")
		if ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", corsExposeHeaders)
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", corsHeaders)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		} else if r.Method == http.MethodOptions {
			http.Error(w, "CORS origin is not allowed", http.StatusForbidden)
			return
		}
		// CORS controls browser access; it is not an authentication mechanism.
		next.ServeHTTP(w, r)
	})
}
