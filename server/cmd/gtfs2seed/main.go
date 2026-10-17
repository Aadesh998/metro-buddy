// gtfs2seed prints metro seed JSON lines for a GTFS feed directory:
//
//	go run ./cmd/gtfs2seed path/to/gtfs > lines.json
//
// Merge the output into internal/metro/seed/<city>.json, then add transfers and aliases by hand.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"metro-buddy/server/internal/metro/gtfs"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gtfs2seed <gtfs-dir>")
		os.Exit(2)
	}
	lines, err := gtfs.Lines(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{"lines": lines}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
