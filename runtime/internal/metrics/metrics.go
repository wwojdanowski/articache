package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	once sync.Once

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "articache",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP artifact requests handled by Articache.",
		},
		[]string{"result"}, // hit|miss|bad_request
	)

	CacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "articache",
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits (served from local disk).",
		},
	)

	CacheMissesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "articache",
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses (redirected upstream).",
		},
	)

	DownloadQueuedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "articache",
			Name:      "downloads_queued_total",
			Help:      "Total number of async download jobs queued.",
		},
	)

	DownloadQueueDroppedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "articache",
			Name:      "downloads_queue_dropped_total",
			Help:      "Total number of async download jobs dropped due to a full queue.",
		},
	)

	DownloadQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "articache",
			Name:      "download_queue_depth",
			Help:      "Current depth of the async download queue.",
		},
	)

	DownloadsInflight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "articache",
			Name:      "downloads_inflight",
			Help:      "Number of downloads currently in progress.",
		},
	)

	DownloadsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "articache",
			Name:      "downloads_total",
			Help:      "Total number of download attempts performed by Articache.",
		},
		[]string{"outcome"}, // success|failure
	)

	DownloadDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "articache",
			Name:      "download_duration_seconds",
			Help:      "Time spent downloading an artifact from the upstream repository.",
			Buckets:   prometheus.DefBuckets,
		},
	)
)

func Register(reg prometheus.Registerer) {
	once.Do(func() {
		reg.MustRegister(
			HTTPRequestsTotal,
			CacheHitsTotal,
			CacheMissesTotal,
			DownloadQueuedTotal,
			DownloadQueueDroppedTotal,
			DownloadQueueDepth,
			DownloadsInflight,
			DownloadsTotal,
			DownloadDurationSeconds,
		)
	})
}

