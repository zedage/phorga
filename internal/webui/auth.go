package webui

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// requirePassword asks for HTTP Basic credentials with the given password
// on every request; the user name is not checked (WEB-020). An empty
// password turns the check off.
func requirePassword(password string, next http.Handler) http.Handler {
	if password == "" {
		return next
	}
	want := sha256.Sum256([]byte(password))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, given, ok := r.BasicAuth()
		got := sha256.Sum256([]byte(given))
		// Hashing first makes the comparison constant-time also for
		// passwords of different length.
		if !ok || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Phorga", charset="UTF-8"`)
			http.Error(w, "Phorga: password required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rejectCrossSite answers 403 to writes that a browser marks as coming from
// another site, so no other web page can change the configuration through
// the visitor's browser (WEB-021).
func rejectCrossSite(next http.Handler) http.Handler {
	return http.NewCrossOriginProtection().Handler(next)
}
