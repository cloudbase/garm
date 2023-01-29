package metrics

import (
	"log"

	"garm/auth"
	"garm/params"
	"garm/runner"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var webhooksReceived *prometheus.CounterVec = nil

// RecordWebhookWithLabels will increment a webhook metric identified by specific
// values. If metrics are disabled, this function is a noop.
func RecordWebhookWithLabels(lvs ...string) error {
	if webhooksReceived == nil {
		// not registered. Noop
		return nil
	}

	counter, err := webhooksReceived.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return errors.Wrap(err, "recording metric")
	}
	counter.Inc()
	return nil
}

func RegisterCollectors(runner *runner.Runner) error {
	if webhooksReceived != nil {
		// Already registered.
		return nil
	}

	garmCollector, err := NewGarmCollector(runner)
	if err != nil {
		return errors.Wrap(err, "getting collector")
	}

	if err := prometheus.Register(garmCollector); err != nil {
		return errors.Wrap(err, "registering collector")
	}

	// metric to count total webhooks received
	// at this point the webhook is not yet authenticated and
	// we don't know if it's meant for us or not
	webhooksReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "garm_webhooks_received",
		Help: "The total number of webhooks received",
	}, []string{"valid", "reason", "hostname", "controller_id"})

	err = prometheus.Register(webhooksReceived)
	if err != nil {
		return errors.Wrap(err, "registering webhooks recv counter")
	}
	return nil
}

func NewGarmCollector(r *runner.Runner) (*GarmCollector, error) {
	controllerInfo, err := r.GetControllerInfo(auth.GetAdminContext())
	if err != nil {
		return nil, errors.Wrap(err, "fetching controller info")
	}
	return &GarmCollector{
		runner: r,
		instanceMetric: prometheus.NewDesc(
			"garm_runner_status",
			"Status of the runner",
			[]string{"name", "status", "runner_status", "pool_owner", "pool_type", "pool_id", "hostname", "controller_id"}, nil,
		),
		healthMetric: prometheus.NewDesc(
			"garm_health",
			"Health of the runner",
			[]string{"hostname", "controller_id"}, nil,
		),
		cachedControllerInfo: controllerInfo,
	}, nil
}

type GarmCollector struct {
	healthMetric         *prometheus.Desc
	instanceMetric       *prometheus.Desc
	runner               *runner.Runner
	cachedControllerInfo params.ControllerInfo
}

func (c *GarmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.instanceMetric
	ch <- c.healthMetric
}

func (c *GarmCollector) Collect(ch chan<- prometheus.Metric) {
	controllerInfo, err := c.runner.GetControllerInfo(auth.GetAdminContext())
	if err != nil {
		log.Printf("failed to get controller info: %s", err)
		return
	}
	c.CollectInstanceMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectHealthMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
}

func (c *GarmCollector) CollectHealthMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	m, err := prometheus.NewConstMetric(
		c.healthMetric,
		prometheus.GaugeValue,
		1,
		hostname,
		controllerID,
	)
	if err != nil {
		log.Printf("error on creating health metric: %s", err)
		return
	}
	ch <- m
}

// CollectInstanceMetric collects the metrics for the runner instances
// reflecting the statuses and the pool they belong to.
func (c *GarmCollector) CollectInstanceMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	instances, err := c.runner.ListAllInstances(ctx)
	if err != nil {
		log.Printf("cannot collect metrics, listing instances: %s", err)
		return
	}

	pools, err := c.runner.ListAllPools(ctx)
	if err != nil {
		log.Printf("listing pools: %s", err)
		// continue anyway
	}

	type poolInfo struct {
		Name string
		Type string
	}

	poolNames := make(map[string]poolInfo)
	for _, pool := range pools {
		if pool.EnterpriseName != "" {
			poolNames[pool.ID] = poolInfo{
				Name: pool.EnterpriseName,
				Type: string(pool.PoolType()),
			}
		} else if pool.OrgName != "" {
			poolNames[pool.ID] = poolInfo{
				Name: pool.OrgName,
				Type: string(pool.PoolType()),
			}
		} else {
			poolNames[pool.ID] = poolInfo{
				Name: pool.RepoName,
				Type: string(pool.PoolType()),
			}
		}
	}

	for _, instance := range instances {

		m, err := prometheus.NewConstMetric(
			c.instanceMetric,
			prometheus.GaugeValue,
			1,
			instance.Name,
			string(instance.Status),
			string(instance.RunnerStatus),
			poolNames[instance.PoolID].Name,
			poolNames[instance.PoolID].Type,
			instance.PoolID,
			hostname,
			controllerID,
		)

		if err != nil {
			log.Printf("cannot collect metrics, creating metric: %s", err)
			continue
		}
		ch <- m
	}
}
