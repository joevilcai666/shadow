//go:build !windows

package daemon

import "os/exec"

func openURL(url string) {
	_ = exec.Command("open", url).Start()
}
