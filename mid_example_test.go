package mid_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/earthboundkid/mid"
)

func Example() {
	// Sample middleware that runs before and after some handlers
	before := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "before,")
			h.ServeHTTP(w, r)
		})
	}
	after := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
			fmt.Fprintln(w, ",after")
		})
	}
	// Environment of some kind
	env := "prod"

	// Example middleware stack
	var mws mid.Stack
	mws.Push(before)                 // Everything gets the before middleware
	mws.PushIf(env == "prod", after) // Prod gets the after middleware

	// ServeMux with different handlers on /a /b and /c
	mux := http.NewServeMux()
	mws.
		Handle(mux, "GET /a", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "a")
		})).
		HandleFunc(mux, "POST /b", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "b")
		}).
		Control(mux, "PUT /c", func(w http.ResponseWriter, r *http.Request) http.Handler {
			fmt.Fprint(w, "c")
			return nil
		})

	// Make sure it all works
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest("GET", "/a", nil))
	mux.ServeHTTP(res, httptest.NewRequest("POST", "/b", nil))
	mux.ServeHTTP(res, httptest.NewRequest("PUT", "/c", nil))
	fmt.Print(res.Body.String())
	// Output:
	// before,a,after
	// before,b,after
	// before,c,after
}
