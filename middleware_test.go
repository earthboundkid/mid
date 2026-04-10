package mid_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/earthboundkid/mid"

	"github.com/carlmjohnson/be"
	"github.com/carlmjohnson/requests"
	"github.com/carlmjohnson/requests/reqtest"
)

func TestMiddleware(t *testing.T) {
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
	be.Equal(t, "123h321", w.Body.String())

	// Work multiple times
	w = httptest.NewRecorder()
	h.ServeHTTP(w, nil)
	be.Equal(t, "123h321", w.Body.String())
}

func TestController(t *testing.T) {
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
	be.Equal(t, "1", w.Body.String())

	cond = false
	w = httptest.NewRecorder()
	c.ServeHTTP(w, nil)
	be.Equal(t, "2", w.Body.String())
}

func TestStack(t *testing.T) {
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
	be.Equal(t, "1h1", w.Body.String())

	w = httptest.NewRecorder()
	g1.ServeHTTP(w, nil)
	be.Equal(t, "2g2", w.Body.String())

	w = httptest.NewRecorder()
	h2.ServeHTTP(w, nil)
	be.Equal(t, "13h31", w.Body.String())

	g2 := mws2.Controller(g)

	w = httptest.NewRecorder()
	g2.ServeHTTP(w, nil)
	be.Equal(t, "2g2", w.Body.String())

	w = httptest.NewRecorder()
	h3.ServeHTTP(w, nil)
	be.Equal(t, "135h531", w.Body.String())
}

func TestHandle(t *testing.T) {
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
	s := httptest.NewServer(mux)
	defer s.Close()
	req := requests.New(reqtest.Server(s))

	// Make sure it all works
	var body string
	be.NilErr(t, req.Path("/a").ToString(&body).Fetch(t.Context()))
	be.Equal(t, "before,a,after", body)

	be.NilErr(t, req.Path("/b").ToString(&body).Fetch(t.Context()))
	be.Equal(t, "before,b,after", body)

	be.NilErr(t, req.Path("/c").ToString(&body).Fetch(t.Context()))
	be.Equal(t, "before,c,after", body)
}
