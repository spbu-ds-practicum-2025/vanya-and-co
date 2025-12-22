package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	paths := []string{
		"./static",
		"static",
		"services/gateway/cmd/server/static",
		"./services/gateway/cmd/server/static",
		"../../static",
		"/app/static",
	}

	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
			fmt.Println("Found static files at:", path)
			return
		} else {
			fmt.Printf("Not found at %s: %v\n", path, err)
		}
	}

	fmt.Println("Static files not found anywhere")
}
