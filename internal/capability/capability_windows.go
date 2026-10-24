//go:build windows

package capability

import (
	"os/exec"
	"strings"
)

func defaultCopy(s string) error {
	cmd := exec.Command("clip")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

func defaultOpenURL(url string) error {
	return runDetached("rundll32", "url.dll,FileProtocolHandler", url)
}
