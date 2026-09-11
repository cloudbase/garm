// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package watcher

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// consumerKind maps a consumer ID to a bounded label value. Several consumer
// IDs embed entity IDs, user IDs or runner names; exporting those verbatim
// would create unbounded label cardinality.
func consumerKind(id string) string {
	prefixes := []string{
		"ws-event-watcher",
		"agent-worker",
		"entity-worker",
		"scaleset-worker",
		"scaleset-controller",
		"pool-manager",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(id, prefix+"-") {
			return prefix
		}
	}
	return id
}

type queueDepthCollector struct {
	desc *prometheus.Desc
}

// NewQueueDepthCollector returns a prometheus collector that reports the
// dispatch queue depth of registered consumers at scrape time, summed per
// consumer kind. A growing queue depth means a consumer is not keeping up
// with the event stream.
func NewQueueDepthCollector() prometheus.Collector {
	return queueDepthCollector{
		desc: prometheus.NewDesc(
			"garm_watcher_consumer_queue_depth",
			"Number of undelivered events queued per watcher consumer kind",
			[]string{"consumer"}, nil,
		),
	}
}

func (q queueDepthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- q.desc
}

func (q queueDepthCollector) Collect(ch chan<- prometheus.Metric) {
	if databaseWatcher == nil {
		return
	}

	depths := make(map[string]int)
	for id, depth := range databaseWatcher.Metrics().ConsumerQueueDepths {
		depths[consumerKind(id)] += depth
	}

	for kind, depth := range depths {
		ch <- prometheus.MustNewConstMetric(q.desc, prometheus.GaugeValue, float64(depth), kind)
	}
}
