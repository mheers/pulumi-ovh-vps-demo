package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAdditionalDiskSetupScriptSyntax(t *testing.T) {
	script := additionalDiskSetupScript("/mnt/disk")
	cmd := exec.Command("sh", "-n")
	cmd.Stdin = strings.NewReader(script)

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated mount script is invalid: %v\n%s", err, output)
	}
	if !strings.Contains(script, `blkid -s TYPE -o value`) {
		t.Fatal("generated mount script must verify the filesystem type before skipping mkfs")
	}
}
