package gtfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinesPicksLongestTripAndParentStationNames(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"routes.txt":     "route_id,route_short_name,route_long_name,route_color\nY,Y,Yellow Line,FFCC00\n",
		"trips.txt":      "route_id,service_id,trip_id,direction_id\nY,wk,short,0\nY,wk,long,0\nY,wk,back,1\n",
		"stops.txt":      "stop_id,stop_name,parent_station\nRC,Rajiv Chowk,\nRC1,Rajiv Chowk Platform 1,RC\nPC,Patel Chowk,\nCS,Central Secretariat,\n",
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nshort,08:00:00,08:00:00,RC1,1\nshort,08:02:00,08:02:00,PC,2\nlong,09:00:00,09:00:00,RC1,1\nlong,09:02:00,09:02:00,PC,2\nlong,09:04:00,09:04:00,CS,3\nback,10:00:00,10:00:00,CS,1\n",
	}
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := Lines(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Color != "#FFCC00" || len(lines[0].Stations) != 3 || lines[0].Stations[0] != "Rajiv Chowk" {
		t.Fatalf("%+v", lines)
	}
}
