// Copyright 2026 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package metrics

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/websocket"
)

const snapshotInterval = 5 * time.Second

// MetricsHub broadcasts pre-aggregated metrics snapshots to all connected
// dashboard WebSocket clients at a fixed interval. It reads from the
// in-memory cache (no database queries) and computes the snapshot once
// for all clients.
type MetricsHub struct {
	hub *websocket.Hub
	ctx context.Context

	quit    chan struct{}
	running bool
	mux     sync.Mutex
}

func NewMetricsHub(ctx context.Context) *MetricsHub {
	return &MetricsHub{
		hub:  websocket.NewHub(ctx),
		ctx:  ctx,
		quit: make(chan struct{}),
	}
}

func (m *MetricsHub) Start() error {
	m.mux.Lock()
	defer m.mux.Unlock()

	if m.running {
		return nil
	}

	if err := m.hub.Start(); err != nil {
		return err
	}

	m.running = true
	go m.tickerLoop()
	return nil
}

func (m *MetricsHub) Stop() error {
	m.mux.Lock()
	defer m.mux.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.quit)
	return m.hub.Stop()
}

// Register adds a client to the hub and sends an immediate snapshot
// so the dashboard doesn't have to wait for the next tick.
func (m *MetricsHub) Register(client *websocket.Client) error {
	if err := m.hub.Register(client); err != nil {
		return err
	}

	// Send immediate snapshot to the new client
	data, err := json.Marshal(BuildSnapshot())
	if err != nil {
		slog.ErrorContext(m.ctx, "failed to marshal initial metrics snapshot", "error", err)
		return nil // Don't fail the registration
	}
	if _, err := client.Write(data); err != nil {
		slog.WarnContext(m.ctx, "failed to send initial metrics snapshot", "error", err)
	}
	return nil
}

func (m *MetricsHub) Unregister(client *websocket.Client) error {
	return m.hub.Unregister(client)
}

func (m *MetricsHub) tickerLoop() {
	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.quit:
			return
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			data, err := json.Marshal(BuildSnapshot())
			if err != nil {
				slog.ErrorContext(m.ctx, "failed to marshal metrics snapshot", "error", err)
				continue
			}
			if _, err := m.hub.Write(data); err != nil {
				slog.DebugContext(m.ctx, "failed to broadcast metrics snapshot", "error", err)
			}
		}
	}
}

// BuildSnapshot reads from the in-memory cache and aggregates data into
// a MetricsSnapshot. This function is safe to call concurrently — the
// cache handles its own locking.
func BuildSnapshot() MetricsSnapshot {
	allEntities := cache.GetAllEntities()
	allPools := cache.GetAllPools()
	allScaleSets := cache.GetAllScaleSets()
	allInstances := cache.GetAllInstancesCache()

	// Count pools per entity
	poolCountByEntity := make(map[string]int, len(allEntities))
	for _, pool := range allPools {
		entityID := pool.RepoID
		if entityID == "" {
			entityID = pool.OrgID
		}
		if entityID == "" {
			entityID = pool.EnterpriseID
		}
		if entityID != "" {
			poolCountByEntity[entityID]++
		}
	}

	// Count scalesets per entity
	scaleSetCountByEntity := make(map[string]int, len(allEntities))
	for _, ss := range allScaleSets {
		entityID := ss.RepoID
		if entityID == "" {
			entityID = ss.OrgID
		}
		if entityID == "" {
			entityID = ss.EnterpriseID
		}
		if entityID != "" {
			scaleSetCountByEntity[entityID]++
		}
	}

	// Build entity metrics
	entities := make([]MetricsEntity, 0, len(allEntities))
	for _, e := range allEntities {
		name := e.Name
		if name == "" {
			name = e.Owner
		}
		if e.EntityType == params.ForgeEntityTypeRepository && e.Owner != "" && e.Name != "" {
			name = e.Owner + "/" + e.Name
		}

		entities = append(entities, MetricsEntity{
			ID:            e.ID,
			Name:          name,
			Type:          string(e.EntityType),
			Endpoint:      e.Credentials.Endpoint.Name,
			PoolCount:     poolCountByEntity[e.ID],
			ScaleSetCount: scaleSetCountByEntity[e.ID],
			Healthy:       e.PoolManagerStatus.IsRunning,
		})
	}

	// Count instances per pool by VM status and runner status
	poolRunnerCounts := make(map[string]map[string]int, len(allPools))
	poolRunnerStatusCounts := make(map[string]map[string]int, len(allPools))
	scaleSetRunnerCounts := make(map[uint]map[string]int, len(allScaleSets))
	scaleSetRunnerStatusCounts := make(map[uint]map[string]int, len(allScaleSets))

	for _, inst := range allInstances {
		status := string(inst.Status)
		runnerStatus := string(inst.RunnerStatus)

		if inst.PoolID != "" {
			if _, ok := poolRunnerCounts[inst.PoolID]; !ok {
				poolRunnerCounts[inst.PoolID] = make(map[string]int)
			}
			poolRunnerCounts[inst.PoolID][status]++

			if runnerStatus != "" {
				if _, ok := poolRunnerStatusCounts[inst.PoolID]; !ok {
					poolRunnerStatusCounts[inst.PoolID] = make(map[string]int)
				}
				poolRunnerStatusCounts[inst.PoolID][runnerStatus]++
			}
		}
		if inst.ScaleSetID > 0 {
			if _, ok := scaleSetRunnerCounts[inst.ScaleSetID]; !ok {
				scaleSetRunnerCounts[inst.ScaleSetID] = make(map[string]int)
			}
			scaleSetRunnerCounts[inst.ScaleSetID][status]++

			if runnerStatus != "" {
				if _, ok := scaleSetRunnerStatusCounts[inst.ScaleSetID]; !ok {
					scaleSetRunnerStatusCounts[inst.ScaleSetID] = make(map[string]int)
				}
				scaleSetRunnerStatusCounts[inst.ScaleSetID][runnerStatus]++
			}
		}
	}

	// Build pool metrics
	pools := make([]MetricsPool, 0, len(allPools))
	for _, p := range allPools {
		counts := poolRunnerCounts[p.ID]
		if counts == nil {
			counts = make(map[string]int)
		}
		rsCounts := poolRunnerStatusCounts[p.ID]
		if rsCounts == nil {
			rsCounts = make(map[string]int)
		}
		pools = append(pools, MetricsPool{
			ID:                 p.ID,
			ProviderName:       p.ProviderName,
			OSType:             string(p.OSType),
			MaxRunners:         p.MaxRunners,
			Enabled:            p.Enabled,
			RepoName:           p.RepoName,
			OrgName:            p.OrgName,
			EnterpriseName:     p.EnterpriseName,
			RunnerCounts:       counts,
			RunnerStatusCounts: rsCounts,
		})
	}

	// Build scale set metrics
	scaleSets := make([]MetricsScaleSet, 0, len(allScaleSets))
	for _, ss := range allScaleSets {
		counts := scaleSetRunnerCounts[ss.ID]
		if counts == nil {
			counts = make(map[string]int)
		}
		rsCounts := scaleSetRunnerStatusCounts[ss.ID]
		if rsCounts == nil {
			rsCounts = make(map[string]int)
		}
		scaleSets = append(scaleSets, MetricsScaleSet{
			ID:                 ss.ID,
			Name:               ss.Name,
			ProviderName:       ss.ProviderName,
			OSType:             string(ss.OSType),
			MaxRunners:         ss.MaxRunners,
			Enabled:            ss.Enabled,
			RepoName:           ss.RepoName,
			OrgName:            ss.OrgName,
			EnterpriseName:     ss.EnterpriseName,
			RunnerCounts:       counts,
			RunnerStatusCounts: rsCounts,
		})
	}

	return MetricsSnapshot{
		Entities:  entities,
		Pools:     pools,
		ScaleSets: scaleSets,
	}
}
