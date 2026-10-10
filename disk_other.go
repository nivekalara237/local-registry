//go:build !unix

package local_registry

// diskFree cannot tell on this platform: the free-space check is skipped.
func diskFree(string) (int64, bool) {
	return 0, false
}
