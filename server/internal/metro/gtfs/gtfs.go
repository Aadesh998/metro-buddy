// Package gtfs converts a GTFS feed into the metro seed format.
package gtfs

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Line struct {
	GTFSRouteID string   `json:"gtfs_route_id"`
	Name        string   `json:"name"`
	Color       string   `json:"color"`
	Stations    []string `json:"stations"`
}

func readCSV(dir, name string) ([]map[string]string, error) {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for i, h := range header {
		header[i] = strings.TrimPrefix(strings.TrimSpace(h), "\uFEFF")
	}
	var out []map[string]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		row := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				row[h] = strings.TrimSpace(rec[i])
			}
		}
		out = append(out, row)
	}
}

// Lines picks, for each route (and direction 0), the trip with the most stops as the
// canonical station order, and names stops by their parent station when one exists.
func Lines(dir string) ([]Line, error) {
	routes, err := readCSV(dir, "routes.txt")
	if err != nil {
		return nil, err
	}
	trips, err := readCSV(dir, "trips.txt")
	if err != nil {
		return nil, err
	}
	stopTimes, err := readCSV(dir, "stop_times.txt")
	if err != nil {
		return nil, err
	}
	stops, err := readCSV(dir, "stops.txt")
	if err != nil {
		return nil, err
	}

	stopName := map[string]string{}
	parent := map[string]string{}
	for _, s := range stops {
		stopName[s["stop_id"]] = s["stop_name"]
		if p := s["parent_station"]; p != "" {
			parent[s["stop_id"]] = p
		}
	}
	nameOf := func(id string) string {
		if p, ok := parent[id]; ok {
			if n := stopName[p]; n != "" {
				return n
			}
		}
		return stopName[id]
	}

	type seqStop struct {
		seq  int
		stop string
	}
	byTrip := map[string][]seqStop{}
	for _, st := range stopTimes {
		n, _ := strconv.Atoi(st["stop_sequence"])
		byTrip[st["trip_id"]] = append(byTrip[st["trip_id"]], seqStop{n, st["stop_id"]})
	}

	best := map[string]string{}
	for _, t := range trips {
		if d := t["direction_id"]; d != "" && d != "0" {
			continue
		}
		rid, tid := t["route_id"], t["trip_id"]
		if len(byTrip[tid]) > len(byTrip[best[rid]]) {
			best[rid] = tid
		}
	}

	var out []Line
	for _, r := range routes {
		tid, ok := best[r["route_id"]]
		if !ok {
			continue
		}
		ss := byTrip[tid]
		sort.Slice(ss, func(i, j int) bool { return ss[i].seq < ss[j].seq })
		var names []string
		for _, s := range ss {
			n := nameOf(s.stop)
			if len(names) == 0 || names[len(names)-1] != n {
				names = append(names, n)
			}
		}
		name := r["route_long_name"]
		if name == "" {
			name = r["route_short_name"]
		}
		color := "#888888"
		if c := r["route_color"]; len(c) == 6 {
			color = "#" + strings.ToUpper(c)
		}
		out = append(out, Line{GTFSRouteID: r["route_id"], Name: name, Color: color, Stations: names})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GTFSRouteID < out[j].GTFSRouteID })
	return out, nil
}
