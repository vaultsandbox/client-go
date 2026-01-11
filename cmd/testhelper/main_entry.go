//go:build !testcoverage

package main

import "os"

func main() {
	if err := run(os.Args, DefaultConfig()); err != nil {
		fatal("%v", err)
	}
}
