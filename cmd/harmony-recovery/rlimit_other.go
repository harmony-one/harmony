//go:build !unix

package main

// checkFDLimit is a no-op on platforms without RLIMIT_NOFILE.
func checkFDLimit(handles int) error { return nil }
