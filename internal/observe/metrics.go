package observe

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	KubeJobsInCronEngineTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "kube_jobs_in_cron_engine_total",
		Help:        "Jobs in queue of Cron Engine",
		ConstLabels: map[string]string{},
	})

	KubeExpiredJobsInCronEngineTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "kube_expired_jobs_in_cron_engine_total",
		Help:        "Expired jobs in queue of Cron Engine",
		ConstLabels: map[string]string{},
	})

	KubeSubmittedJobsInCronEngineTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "kube_submitted_jobs_in_cron_engine_total",
		Help:        "Submitted jobs in queue of Cron Engine",
		ConstLabels: map[string]string{},
	})

	KubeSuccessfulJobsInCronEngineTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "kube_successful_jobs_in_cron_engine_total",
		Help:        "Successful jobs in queue of Cron Engine",
		ConstLabels: map[string]string{},
	})

	KubeFailedJobsInCronEngineTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "kube_failed_jobs_in_cron_engine_total",
		Help:        "Failed jobs in queue of Cron Engine",
		ConstLabels: map[string]string{},
	})
)

func init() {
	metrics.Registry.MustRegister(KubeJobsInCronEngineTotal)
	metrics.Registry.MustRegister(KubeSubmittedJobsInCronEngineTotal)
	metrics.Registry.MustRegister(KubeSuccessfulJobsInCronEngineTotal)
	metrics.Registry.MustRegister(KubeFailedJobsInCronEngineTotal)
	metrics.Registry.MustRegister(KubeExpiredJobsInCronEngineTotal)
}
