package adb

// Result is the outcome of a single adb invocation.
type Result struct {
	// Stdout is the raw captured standard output. It is binary-safe (for
	// example a PNG from screencap).
	Stdout []byte
	// Stderr is the raw captured standard error output.
	Stderr []byte
	// Code is the process exit code (-1 if the process never ran).
	Code int
}

// StdoutString returns the standard output as a string.
func (r Result) StdoutString() string { return string(r.Stdout) }

// StderrString returns the standard error output as a string.
func (r Result) StderrString() string { return string(r.Stderr) }
