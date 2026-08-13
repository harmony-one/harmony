//go:build !unix

package rodb

// ProbeLiveWriter is unsupported on this platform; the result is unknown.
func ProbeLiveWriter(dir string) (running bool, known bool) {
	return false, false
}
