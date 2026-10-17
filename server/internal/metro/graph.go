package metro

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

type edge struct {
	to   uuid.UUID
	line uuid.UUID
}

type Graph struct {
	adj   map[uuid.UUID][]edge
	lines map[uuid.UUID][]uuid.UUID
}

type Service struct {
	DB     *gorm.DB
	mu     sync.Mutex
	graphs map[uuid.UUID]*Graph
}

func (s *Service) Graph(ctx context.Context, cityID uuid.UUID) (*Graph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.graphs[cityID]; ok {
		return g, nil
	}
	g, err := load(s.DB.WithContext(ctx), cityID)
	if err != nil {
		return nil, err
	}
	if s.graphs == nil {
		s.graphs = map[uuid.UUID]*Graph{}
	}
	s.graphs[cityID] = g
	return g, nil
}

func (s *Service) Invalidate() {
	s.mu.Lock()
	s.graphs = nil
	s.mu.Unlock()
}

func load(db *gorm.DB, cityID uuid.UUID) (*Graph, error) {
	var rows []struct {
		LineID    uuid.UUID
		StationID uuid.UUID
		Seq       int16
	}
	err := db.Table("line_stations").Select("line_stations.line_id, line_stations.station_id, line_stations.seq").
		Joins("JOIN metro_lines ON metro_lines.id = line_stations.line_id").
		Where("metro_lines.city_id = ?", cityID).
		Order("metro_lines.gtfs_route_id, line_stations.seq").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	g := &Graph{adj: map[uuid.UUID][]edge{}, lines: map[uuid.UUID][]uuid.UUID{}}
	for i, r := range rows {
		g.addLine(r.StationID, r.LineID)
		if i > 0 && rows[i-1].LineID == r.LineID && rows[i-1].Seq == r.Seq-1 {
			prev := rows[i-1].StationID
			g.adj[prev] = append(g.adj[prev], edge{to: r.StationID, line: r.LineID})
			g.adj[r.StationID] = append(g.adj[r.StationID], edge{to: prev, line: r.LineID})
		}
	}
	var transfers []model.StationTransfer
	if err := db.Table("station_transfers").
		Joins("JOIN stations ON stations.id = station_transfers.from_station_id").
		Where("stations.city_id = ?", cityID).Find(&transfers).Error; err != nil {
		return nil, err
	}
	for _, t := range transfers {
		g.adj[t.FromStationID] = append(g.adj[t.FromStationID], edge{to: t.ToStationID})
	}
	return g, nil
}

func (g *Graph) addLine(station, line uuid.UUID) {
	for _, l := range g.lines[station] {
		if l == line {
			return
		}
	}
	g.lines[station] = append(g.lines[station], line)
}

func (g *Graph) Has(station uuid.UUID) bool {
	_, ok := g.lines[station]
	return ok
}

func (g *Graph) LinesAt(station uuid.UUID) []uuid.UUID { return g.lines[station] }

// Path returns the fewest-stops station sequence from -> to (inclusive), or nil if unreachable.
func (g *Graph) Path(from, to uuid.UUID) []uuid.UUID {
	if !g.Has(from) || !g.Has(to) {
		return nil
	}
	if from == to {
		return []uuid.UUID{from}
	}
	prev := map[uuid.UUID]uuid.UUID{from: from}
	queue := []uuid.UUID{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range g.adj[cur] {
			if _, seen := prev[e.to]; seen {
				continue
			}
			prev[e.to] = cur
			if e.to == to {
				path := []uuid.UUID{to}
				for n := cur; n != from; n = prev[n] {
					path = append(path, n)
				}
				path = append(path, from)
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			queue = append(queue, e.to)
		}
	}
	return nil
}

// PathLines lists the lines ridden along a path, in order, without repeats.
func (g *Graph) PathLines(path []uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for i := 1; i < len(path); i++ {
		for _, e := range g.adj[path[i-1]] {
			if e.to == path[i] && e.line != uuid.Nil {
				if len(out) == 0 || out[len(out)-1] != e.line {
					out = append(out, e.line)
				}
				break
			}
		}
	}
	return out
}

var ErrNoRoute = httpx.Unprocessable("no_route", "no metro route between those stations")

// Route resolves both stations to their city graph and returns the BFS path and lines ridden.
func (s *Service) Route(ctx context.Context, from, to uuid.UUID) ([]uuid.UUID, []uuid.UUID, error) {
	var st []model.Station
	if err := s.DB.WithContext(ctx).Select("id", "city_id").Where("id IN ?", []uuid.UUID{from, to}).Find(&st).Error; err != nil {
		return nil, nil, err
	}
	if (from == to && len(st) != 1) || (from != to && len(st) != 2) || st[0].CityID != st[len(st)-1].CityID {
		return nil, nil, ErrNoRoute
	}
	g, err := s.Graph(ctx, st[0].CityID)
	if err != nil {
		return nil, nil, err
	}
	path := g.Path(from, to)
	if path == nil {
		return nil, nil, ErrNoRoute
	}
	return path, g.PathLines(path), nil
}
