//go:build !darwin && !windows

package capability

import (
	"os/exec"
	"strings"
)

func defaultCopy(s string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

func defaultOpenURL(url string) error {
	return runDetached("xdg-open", url)
}
