// Package mid has extremely mid middleware helpers.
package mid

import (
	"net/http"
	"slices"
)

// Controller implements the pattern in
// https://choly.ca/post/go-experiments-with-handler/
type Controller func(w http.ResponseWriter, r *http.Request) http.Handler

func (c Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h := c(w, r); h != nil {
		h.ServeHTTP(w, r)
	}
}

// Middleware is any function that wraps an http.Handler returning a new http.Handler.
type Middleware = func(h http.Handler) http.Handler

// Stack is a slice of Middleware.
type Stack []Middleware

func (stack Stack) Clone() Stack {
	return slices.Clone(stack)
}

// Push adds Middleware to end of the stack.
func (stack *Stack) Push(mw ...Middleware) {
	*stack = append(*stack, mw...)
}

// PushIf adds Middleware to end of the stack if cond is true.
func (stack *Stack) PushIf(cond bool, mw ...Middleware) {
	if cond {
		stack.Push(mw...)
	}
}

// With clones the stack and pushes to the cloned stack.
func (stack *Stack) With(mw ...Middleware) Stack {
	return append(slices.Clip(*stack), mw...)
}

// Handler builds an http.Handler
// that applies all the middleware in the stack
// before calling h.
//
// Handler is itself a Middleware.
func (stack Stack) Handler(h http.Handler) http.Handler {
	for i := len(stack) - 1; i >= 0; i-- {
		m := (stack)[i]
		h = m(h)
	}
	return h
}

// HandlerFunc builds an http.Handler
// that applies all the middleware in the stack
// before calling f.
func (stack Stack) HandlerFunc(f http.HandlerFunc) http.Handler {
	return stack.Handler(f)
}

// Controller builds an http.Handler
// that applies all the middleware in the stack
// before calling c.
func (stack Stack) Controller(c Controller) http.Handler {
	return stack.Handler(c)
}
