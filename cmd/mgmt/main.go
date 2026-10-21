// Command mgmt joins weekly markdown notes with competency resources.
package main

import (
	"fmt"
	"os"

	"github.com/uatec/mgmt/internal/cli"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mgmt: %v\n", err)
		os.Exit(2)
	}
	os.Exit(cli.Run(os.Args[1:], cwd, os.Stdout, os.Stderr))
}
