package handlers

import "net/http"

// cors is deliberately permissive (Allow-Origin: *) because this API issues
// JWTs via the response body rather than cookies, so there's no session
// credential a malicious page could ride along with a cross-origin request.
// For a real deployment, replace "*" with your actual frontend origin(s).
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
