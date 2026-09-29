package mid_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/earthboundkid/mid"

	"github.com/carlmjohnson/requests"
	"github.com/carlmjohnson/requests/reqtest"
	"github.com/earthboundkid/assert"
)

func TestMiddleware(t *testing.T) {
	be := assert.Continues(t)
	mws := mid.Stack{
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("1"))
				h.ServeHTTP(w, r)
				w.Write([]byte("1"))
			})
		},
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("2"))
				h.ServeHTTP(w, r)
				w.Write([]byte("2"))
			})
		},
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("3"))
				h.ServeHTTP(w, r)
				w.Write([]byte("3"))
			})
		},
	}

	h := mws.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("h"))
	})

	// Be resiliant to mutation of the stack
	mws[0] = nil

	// Work once
	w := httptest.NewRecorder()
	h.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "123h321")

	// Work multiple times
	w = httptest.NewRecorder()
	h.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "123h321")
}

func TestController(t *testing.T) {
	be := assert.Continues(t)
	cond := true
	c := mid.Controller(func(w http.ResponseWriter, r *http.Request) http.Handler {
		if cond {
			io.WriteString(w, "1")
			return nil
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "2")
		})
	})
	w := httptest.NewRecorder()
	c.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "1")

	cond = false
	w = httptest.NewRecorder()
	c.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "2")
}

func TestStack(t *testing.T) {
	be := assert.Continues(t)
	// Two basic handlers that just return h or g
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("h"))
	}

	g := func(w http.ResponseWriter, r *http.Request) http.Handler {
		w.Write([]byte("g"))
		return nil
	}

	// Wrap it in a Stack
	mws1 := mid.Stack{
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("1"))
				h.ServeHTTP(w, r)
				w.Write([]byte("1"))
			})
		},
	}

	h1 := mws1.HandlerFunc(h)

	// Test cloning by mutating the original
	mws2 := mws1.Clone()

	mws2[0] = func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("2"))
			h.ServeHTTP(w, r)
			w.Write([]byte("2"))
		})
	}

	g1 := mws2.Controller(g)

	// Test PushIf by adding new stuff
	mws1.PushIf(true, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("3"))
			h.ServeHTTP(w, r)
			w.Write([]byte("3"))
		})
	})

	mws2.PushIf(false, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("4"))
			h.ServeHTTP(w, r)
			w.Write([]byte("4"))
		})
	})

	// Test With to see if original is the same
	mws3 := mws1.With(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("5"))
			h.ServeHTTP(w, r)
			w.Write([]byte("5"))
		})
	})

	h2 := mws1.HandlerFunc(h)
	h3 := mws3.HandlerFunc(h)

	w := httptest.NewRecorder()
	h1.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "1h1")

	w = httptest.NewRecorder()
	g1.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "2g2")

	w = httptest.NewRecorder()
	h2.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "13h31")

	g2 := mws2.Controller(g)

	w = httptest.NewRecorder()
	g2.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "2g2")

	w = httptest.NewRecorder()
	h3.ServeHTTP(w, nil)
	be.Equal(w.Body.String(), "135h531")
}

func TestHandle(t *testing.T) {
	be := assert.Continues(t)
	// Middleware that runs before and after some handlers
	mws := mid.Stack{
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("before,"))
				h.ServeHTTP(w, r)
			})
		},
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.ServeHTTP(w, r)
				w.Write([]byte(",after"))
			})
		},
	}

	// ServeMux with different handlers on /a /b and /c
	mux := http.NewServeMux()
	mws.
		Handle(mux, "/a", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("a"))
		})).
		HandleFunc(mux, "/b", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("b"))
		}).
		Control(mux, "/c", func(w http.ResponseWriter, r *http.Request) http.Handler {
			w.Write([]byte("c"))
			return nil
		})

	// Setup a test server
	s := httptest.NewTestServer(t, mux)
	req := requests.New(reqtest.Server(s))

	// Make sure it all works
	var body string
	be.NilError(req.Path("/a").ToString(&body).Fetch(t.Context()))
	be.Equal(body, "before,a,after")

	be.NilError(req.Path("/b").ToString(&body).Fetch(t.Context()))
	be.Equal(body, "before,b,after")

	be.NilError(req.Path("/c").ToString(&body).Fetch(t.Context()))
	be.Equal(body, "before,c,after")
}
