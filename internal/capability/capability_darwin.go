//go:build darwin

package capability

import (
	"os/exec"
	"strings"
)

func defaultCopy(s string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

func defaultOpenURL(url string) error {
	return runDetached("open", url)
}
