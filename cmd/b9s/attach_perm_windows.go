//go:build windows

package main

import "os"

// normalFileMode returns 0666: Windows has no umask, and os.Chmod there
// only toggles the read-only attribute from the 0200 write bit, so there is
// no masked value to compute or replicate.
func normalFileMode() os.FileMode {
	return 0666
}
