// test-integration refuses a green result when PostgreSQL tests were skipped.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"os/exec"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	_ = godotenv.Load()
	for _, key := range []string{"TEST_DATABASE_URL", "TRAINING_TEST_DATABASE_URL"} {
		if os.Getenv(key) == "" {
			return fmt.Errorf("%s is required; use a dedicated PostgreSQL test database with schema permissions", key)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-json", "./internal/auth/repository/postgres", "./internal/core/training/repository/postgres", "./internal/core/training/handler", "./internal/core/dashboard", "./internal/app", "./cmd/migrate")
	cmd.Stderr = os.Stderr
	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	passed, skipped := 0, 0
	for scanner.Scan() {
		var event struct{ Action, Package, Test, Output string }
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		switch event.Action {
		case "pass":
			if event.Test != "" {
				passed++
			} else {
				fmt.Println("PASS", event.Package)
			}
		case "skip":
			skipped++
			fmt.Println("SKIP", event.Package, event.Test)
		case "output":
			fmt.Print(event.Output)
		}
	}
	scanErr := scanner.Err()
	err = cmd.Wait()
	if scanErr != nil {
		return scanErr
	}
	if err != nil {
		return err
	}
	if skipped > 0 || passed == 0 {
		return fmt.Errorf("integration incomplete: %d passed, %d skipped", passed, skipped)
	}
	fmt.Printf("Integration passed: %d tests, no skips.\n", passed)
	return nil
}
