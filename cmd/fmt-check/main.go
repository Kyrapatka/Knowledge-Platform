// fmt-check is non-mutating and checks only tracked Go source files.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"os/exec"
)

func main() {
	files, err := exec.Command("git", "ls-files", "-z", "--", "*.go").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "list tracked Go files:", err)
		os.Exit(1)
	}
	failed := false
	for _, name := range bytes.Split(files, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		raw, err := os.ReadFile(string(name))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
			continue
		}
		formatted, err := format.Source(raw)
		if err != nil || !bytes.Equal(raw, formatted) {
			fmt.Println(string(name))
			failed = true
		}
	}
	if failed {
		fmt.Fprintln(os.Stderr, "Go formatting differs; run make fmt and review changes.")
		os.Exit(1)
	}
	fmt.Println("Tracked Go formatting: PASS")
}
