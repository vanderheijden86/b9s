//go:build !windows

package main

import (
	"os"
	"syscall"
)

// normalFileMode returns the permission bits a fresh file created via
// open(2) with a requested mode of 0666 would get: the kernel applies the
// process umask to that request automatically, but neither os.Link nor
// os.Rename touch permission bits, so a file placed by linking or renaming
// an os.CreateTemp file (always mode 0600) needs this mask applied
// explicitly to end up with the same result as a normal create.
func normalFileMode() os.FileMode {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return os.FileMode(0666 &^ old)
}
