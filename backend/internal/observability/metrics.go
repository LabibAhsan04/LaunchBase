package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Metrics is a small in-process Prometheus text exposition.
type Metrics struct {
	httpRequests     sync.Map // key method|path|code -> *atomic.Int64
	httpDurationMS   sync.Map // key method|path -> *hist
	queueDepth       atomic.Int64
	jobDurationMS    sync.Map // key outcome -> *hist
	deployments      sync.Map // key outcome -> *atomic.Int64
	activeContainers atomic.Int64
	buildFailures    atomic.Int64
}

type hist struct {
	count atomic.Int64
	sum   atomic.Int64
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) ObserveHTTP(method, path string, code int, durationMS int64) {
	key := method + "|" + path + "|" + fmt.Sprintf("%d", code)
	m.incMap(&m.httpRequests, key)
	hkey := method + "|" + path
	m.observeHist(&m.httpDurationMS, hkey, durationMS)
}

func (m *Metrics) SetQueueDepth(n int64) { m.queueDepth.Store(n) }

func (m *Metrics) ObserveJob(outcome string, durationMS int64) {
	m.observeHist(&m.jobDurationMS, outcome, durationMS)
	m.incMap(&m.deployments, outcome)
	if outcome == "failed" {
		m.buildFailures.Add(1)
	}
}

func (m *Metrics) SetActiveContainers(n int64) { m.activeContainers.Store(n) }

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var b strings.Builder
		b.WriteString("# HELP launchbase_http_requests_total HTTP requests by method, path, and status\n")
		b.WriteString("# TYPE launchbase_http_requests_total counter\n")
		m.httpRequests.Range(func(k, v any) bool {
			parts := strings.Split(k.(string), "|")
			if len(parts) != 3 {
				return true
			}
			fmt.Fprintf(&b, "launchbase_http_requests_total{method=%q,path=%q,code=%q} %d\n",
				parts[0], parts[1], parts[2], v.(*atomic.Int64).Load())
			return true
		})
		b.WriteString("# HELP launchbase_http_request_duration_ms_sum HTTP request duration milliseconds\n")
		b.WriteString("# TYPE launchbase_http_request_duration_ms_sum counter\n")
		b.WriteString("# HELP launchbase_http_request_duration_ms_count HTTP request duration samples\n")
		b.WriteString("# TYPE launchbase_http_request_duration_ms_count counter\n")
		m.writeHist(&b, "launchbase_http_request_duration_ms", &m.httpDurationMS, func(key string) string {
			parts := strings.Split(key, "|")
			if len(parts) != 2 {
				return ""
			}
			return fmt.Sprintf("method=%q,path=%q", parts[0], parts[1])
		})
		fmt.Fprintf(&b, "# HELP launchbase_queue_depth Pending outbox/queue depth estimate\n")
		fmt.Fprintf(&b, "# TYPE launchbase_queue_depth gauge\n")
		fmt.Fprintf(&b, "launchbase_queue_depth %d\n", m.queueDepth.Load())
		b.WriteString("# HELP launchbase_job_duration_ms_sum Worker job duration milliseconds\n")
		b.WriteString("# TYPE launchbase_job_duration_ms_sum counter\n")
		b.WriteString("# HELP launchbase_job_duration_ms_count Worker job duration samples\n")
		b.WriteString("# TYPE launchbase_job_duration_ms_count counter\n")
		m.writeHist(&b, "launchbase_job_duration_ms", &m.jobDurationMS, func(key string) string {
			return fmt.Sprintf("outcome=%q", key)
		})
		b.WriteString("# HELP launchbase_deployments_total Deployment outcomes\n")
		b.WriteString("# TYPE launchbase_deployments_total counter\n")
		m.deployments.Range(func(k, v any) bool {
			fmt.Fprintf(&b, "launchbase_deployments_total{outcome=%q} %d\n", k.(string), v.(*atomic.Int64).Load())
			return true
		})
		fmt.Fprintf(&b, "# HELP launchbase_active_containers Ready containers tracked by worker samples\n")
		fmt.Fprintf(&b, "# TYPE launchbase_active_containers gauge\n")
		fmt.Fprintf(&b, "launchbase_active_containers %d\n", m.activeContainers.Load())
		fmt.Fprintf(&b, "# HELP launchbase_build_failures_total Failed builds\n")
		fmt.Fprintf(&b, "# TYPE launchbase_build_failures_total counter\n")
		fmt.Fprintf(&b, "launchbase_build_failures_total %d\n", m.buildFailures.Load())
		_, _ = w.Write([]byte(b.String()))
	})
}

func (m *Metrics) incMap(mp *sync.Map, key string) {
	actual, _ := mp.LoadOrStore(key, &atomic.Int64{})
	actual.(*atomic.Int64).Add(1)
}

func (m *Metrics) observeHist(mp *sync.Map, key string, ms int64) {
	actual, _ := mp.LoadOrStore(key, &hist{})
	h := actual.(*hist)
	h.count.Add(1)
	h.sum.Add(ms)
}

func (m *Metrics) writeHist(b *strings.Builder, name string, mp *sync.Map, labels func(string) string) {
	keys := []string{}
	mp.Range(func(k, _ any) bool {
		keys = append(keys, k.(string))
		return true
	})
	sort.Strings(keys)
	for _, key := range keys {
		v, _ := mp.Load(key)
		h := v.(*hist)
		lbl := labels(key)
		if lbl == "" {
			continue
		}
		fmt.Fprintf(b, "%s_sum{%s} %d\n", name, lbl, h.sum.Load())
		fmt.Fprintf(b, "%s_count{%s} %d\n", name, lbl, h.count.Load())
	}
}
