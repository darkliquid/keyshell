// Command keyshell is the host binary: configuration, theme, in-process
// backends, the launcher, the capability broker, the overlay renderer, and
// the IPC socket. The command surface lives in internal/cli; see
// `keyshell help` for the full list of commands and flags.
package main

import (
	"os"

	"github.com/darkliquid/keyshell/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args))
}
