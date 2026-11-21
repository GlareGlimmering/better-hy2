// Command trc-maintenance switches one supported test release pair in place.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"h-ui/upgrade"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "--help" {
		fmt.Fprintln(os.Stderr, "Usage: sudo trc-maintenance <v2.3.1-trc.2 asset-directory>")
		fmt.Fprintln(os.Stderr, "Switches a running standard v2.3.1-trc.1 installation to the exact trc.2 release pair.")
		if len(os.Args) == 2 && os.Args[1] == "--help" {
			return
		}
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := upgrade.MigrateTRC2(ctx, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
