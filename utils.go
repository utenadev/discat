package main

import "os"

func isStdin() bool {
	f, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeNamedPipe != 0
}
