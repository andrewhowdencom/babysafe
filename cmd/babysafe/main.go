// Package main is the entrypoint for the babysafe binary.
//
// babysafe captures Linux input devices entirely so that a small human
// (e.g. a baby) can sit at the keyboard without breaking anything. The
// application layer (CLI, configuration, dependency wiring) lives under
// internal/; the underlying input-grab primitives live under pkg/capture
// so they can be reused as a Go library.
//
// # Linux only
//
// babysafe is intended to be run only on Linux. The pkg/capture package
// uses Linux-specific ioctls (EVIOCGRAB) and will not work on other
// operating systems. Cross-compilation is technically possible (the
// dependencies build on any platform) but is not supported; attempting
// to run on a non-Linux kernel will fail at the first ioctl.
package main

import (
	"fmt"
	"os"

	"github.com/andrewhowdencom/babysafe/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
