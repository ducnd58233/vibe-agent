// Package domain describes what executes an agent node.
//
// An agent node is the one kind of graph node the runtime cannot produce
// evidence for on its own: something has to do the work. Until now that
// something was always a host CLI, reached through two copies of the same
// spawn, one in the web composer and one in the routing eval.
//
// The port names the thing rather than the mechanism, so a second
// implementation can arrive without either caller learning about it. The
// runtime keeps owning the outer loop either way: a runner answers one request
// and returns, and nothing here decides what runs next.
package domain

import "context"

// Request is one turn of work to hand a runner.
type Request struct {
	// Prompt is the whole input. Runners differ on whether it travels as an
	// argument or on stdin, which is a property of the runner, not of the work.
	Prompt string
	// Model and Mode are honoured where a runner documents them and ignored
	// where it does not. Silently ignoring is deliberate: refusing would make
	// every caller learn which runner accepts what.
	Model string
	Mode  string
}

// Response is what a runner produced.
type Response struct {
	// Text is the runner's output, unparsed. Callers that need structure parse
	// it themselves, because the shapes differ per host and per version.
	Text string
	// Stderr is what the runner complained about. Captured rather than passed
	// through, so a host that warns on every call does not bury the result, and
	// still available when the call actually fails.
	Stderr string
}

// Runner turns a request into a response.
type Runner interface {
	// Name identifies the runner in a log or an error. Not a stable id.
	Name() string
	Run(ctx context.Context, req Request) (Response, error)
}
