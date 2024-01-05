package metrics

import (
	"log/slog"

	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const metricsNamespace = "garm_"
const metricsRunnerSubsystem = "runner_"
const metricsPoolSubsystem = "pool_"
const metricsProviderSubsystem = "provider_"
const metricsOrganizationSubsystem = "organization_"
const metricsRepositorySubsystem = "repository_"
const metricsEnterpriseSubsystem = "enterprise_"
const metricsWebhookSubsystem = "webhook_"

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
		Name: metricsNamespace + metricsWebhookSubsystem + "received",
		Help: "The total number of webhooks received",
	}, []string{"valid", "reason", "hostname", "controller_id"})

	err = prometheus.Register(webhooksReceived)
	if err != nil {
		return errors.Wrap(err, "registering webhooks recv counter")
	}
	return nil
}

type GarmCollector struct {
	healthMetric   *prometheus.Desc
	instanceMetric *prometheus.Desc

	// pool metrics
	poolInfo             *prometheus.Desc
	poolStatus           *prometheus.Desc
	poolMaxRunners       *prometheus.Desc
	poolMinIdleRunners   *prometheus.Desc
	poolBootstrapTimeout *prometheus.Desc

	// provider metrics
	providerInfo *prometheus.Desc

	organizationInfo              *prometheus.Desc
	organizationPoolManagerStatus *prometheus.Desc
	repositoryInfo                *prometheus.Desc
	repositoryPoolManagerStatus   *prometheus.Desc
	enterpriseInfo                *prometheus.Desc
	enterprisePoolManagerStatus   *prometheus.Desc

	runner               *runner.Runner
	cachedControllerInfo params.ControllerInfo
}

func NewGarmCollector(r *runner.Runner) (*GarmCollector, error) {
	controllerInfo, err := r.GetControllerInfo(auth.GetAdminContext())
	if err != nil {
		return nil, errors.Wrap(err, "fetching controller info")
	}
	return &GarmCollector{
		runner: r,
		instanceMetric: prometheus.NewDesc(
			metricsNamespace+metricsRunnerSubsystem+"status",
			"Status of the runner",
			[]string{"name", "status", "runner_status", "pool_owner", "pool_type", "pool_id", "hostname", "controller_id", "provider"}, nil,
		),
		healthMetric: prometheus.NewDesc(
			metricsNamespace+"health",
			"Health of the runner",
			[]string{"hostname", "controller_id"}, nil,
		),
		poolInfo: prometheus.NewDesc(
			metricsNamespace+metricsPoolSubsystem+"info",
			"Information of the pool",
			[]string{"id", "image", "flavor", "prefix", "os_type", "os_arch", "tags", "provider", "pool_owner", "pool_type"}, nil,
		),
		poolStatus: prometheus.NewDesc(
			metricsNamespace+metricsPoolSubsystem+"status",
			"Status of the pool",
			[]string{"id", "enabled"}, nil,
		),
		poolMaxRunners: prometheus.NewDesc(
			metricsNamespace+metricsPoolSubsystem+"max_runners",
			"Max runners of the pool",
			[]string{"id"}, nil,
		),
		poolMinIdleRunners: prometheus.NewDesc(
			metricsNamespace+metricsPoolSubsystem+"min_idle_runners",
			"Min idle runners of the pool",
			[]string{"id"}, nil,
		),
		poolBootstrapTimeout: prometheus.NewDesc(
			metricsNamespace+metricsPoolSubsystem+"bootstrap_timeout",
			"Bootstrap timeout of the pool",
			[]string{"id"}, nil,
		),
		providerInfo: prometheus.NewDesc(
			metricsNamespace+metricsProviderSubsystem+"info",
			"Info of the provider",
			[]string{"name", "type", "description"}, nil,
		),
		organizationInfo: prometheus.NewDesc(
			metricsNamespace+metricsOrganizationSubsystem+"info",
			"Info of the organization",
			[]string{"name", "id"}, nil,
		),
		organizationPoolManagerStatus: prometheus.NewDesc(
			metricsNamespace+metricsOrganizationSubsystem+"pool_manager_status",
			"Status of the organization pool manager",
			[]string{"name", "id", "running"}, nil,
		),
		repositoryInfo: prometheus.NewDesc(
			metricsNamespace+metricsRepositorySubsystem+"info",
			"Info of the organization",
			[]string{"name", "owner", "id"}, nil,
		),
		repositoryPoolManagerStatus: prometheus.NewDesc(
			metricsNamespace+metricsRepositorySubsystem+"pool_manager_status",
			"Status of the repository pool manager",
			[]string{"name", "id", "running"}, nil,
		),
		enterpriseInfo: prometheus.NewDesc(
			metricsNamespace+metricsEnterpriseSubsystem+"info",
			"Info of the organization",
			[]string{"name", "id"}, nil,
		),
		enterprisePoolManagerStatus: prometheus.NewDesc(
			metricsNamespace+metricsEnterpriseSubsystem+"pool_manager_status",
			"Status of the enterprise pool manager",
			[]string{"name", "id", "running"}, nil,
		),

		cachedControllerInfo: controllerInfo,
	}, nil
}

func (c *GarmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.instanceMetric
	ch <- c.healthMetric
	ch <- c.poolInfo
	ch <- c.poolStatus
	ch <- c.poolMaxRunners
	ch <- c.poolMinIdleRunners
	ch <- c.providerInfo
	ch <- c.organizationInfo
	ch <- c.organizationPoolManagerStatus
	ch <- c.enterpriseInfo
	ch <- c.enterprisePoolManagerStatus
}

func (c *GarmCollector) Collect(ch chan<- prometheus.Metric) {
	controllerInfo, err := c.runner.GetControllerInfo(auth.GetAdminContext())
	if err != nil {
		slog.With(slog.Any("error", err)).Error("failed to get controller info")
		return
	}

	c.CollectInstanceMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectHealthMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectPoolMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectProviderMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectOrganizationMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectRepositoryMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectEnterpriseMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
}
