package blitzkrieg

import (
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

type metricsDef struct {
	sync     sync.RWMutex
	registry metrics.Registry
	current  int
	skipped  metrics.Counter
	busy     metrics.Counter
	all      *metricsSegment
	segments []*metricsSegment
	config   *Config

	sections map[string]childSegment
}

func newMetricsDef(c *Config, hit HitSegment) *metricsDef {
	r := metrics.NewRegistry()
	m := &metricsDef{
		registry: r,
		sections: map[string]childSegment{},
		busy:     metrics.NewRegisteredCounter("busy", r),
		skipped:  metrics.NewRegisteredCounter("skipped", r),
		config:   c,
	}
	m.all = m.newMetricsSegment(hit)
	return m
}

func (m *metricsDef) logBusy(segment int) {
	m.sync.Lock()
	defer m.sync.Unlock()
	m.all.busy.Update(m.busy.Count())
	m.segments[segment].busy.Update(m.busy.Count())
}

func (m *metricsDef) logSkip() {
	m.skipped.Inc(1)
}

func (m *metricsDef) currentSegment() int {
	m.sync.RLock()
	defer m.sync.RUnlock()
	return m.current
}

func (m *metricsDef) logStart(segment int) {
	m.sync.Lock()
	defer m.sync.Unlock()
	m.all.logStart()
	m.segments[segment].logStart()
}

func (m *metricsDef) logSection(rate float64, section string, segment int, hit HitSegment, update func(*metricsSegment, *metricsDef)) {
	m.sync.Lock()
	defer m.sync.Unlock()

	if child, ok := m.sections[section]; ok {
		update(child.getSegment(segment), m)
		return
	}

	child := m.newSubMetrics(section, segment, hit)
	update(child.getSegment(segment), m)
}

func (m *metricsDef) logFinish(segment int, status string, elapsed time.Duration, success bool) {
	m.sync.Lock()
	defer m.sync.Unlock()
	m.all.logFinish(status, elapsed, success)
	m.segments[segment].logFinish(status, elapsed, success)
}

func (m *metricsDef) addSegment(rate HitSegment) {
	m.sync.Lock()
	defer m.sync.Unlock()
	if len(m.segments) > 0 {
		m.segments[m.current].end = time.Now()
	}
	m.segments = append(m.segments, m.newMetricsSegment(rate))
	m.current = len(m.segments) - 1
}

func (m *metricsDef) newMetricsItem() *metricsItem {
	return &metricsItem{
		start:   metrics.NewRegisteredCounter("start", m.registry),
		finish:  metrics.NewRegisteredTimer("finish", m.registry),
		success: metrics.NewRegisteredCounter("success", m.registry),
		fail:    metrics.NewRegisteredCounter("fail", m.registry),
	}
}

func (m *metricsDef) newSubMetrics(section string, segment int, hit HitSegment) *childSegment {
	child := childSegment{
		root:     m,
		key:      section,
		segments: map[int]*metricsSegment{},
	}

	child.addSegment(segment, hit)
	m.sections[section] = child
	return &child
}

func (m *metricsDef) newMetricsSegment(hit HitSegment) *metricsSegment {
	return &metricsSegment{
		def:    m,
		hit:    hit,
		rate:   hit.Rate,
		total:  m.newMetricsItem(),
		status: map[string]*metricsItem{},
		busy:   metrics.NewRegisteredHistogram("busy", m.registry, metrics.NewExpDecaySample(1028, 0.015)),
		start:  time.Now(),
	}
}

type metricsSegment struct {
	sync   sync.RWMutex
	def    *metricsDef
	hit    HitSegment
	rate   float64
	busy   metrics.Histogram
	total  *metricsItem
	status map[string]*metricsItem
	start  time.Time
	end    time.Time
}

func (m *metricsSegment) duration() time.Duration {
	if m.end == (time.Time{}) {
		return time.Since(m.start)
	}
	return m.end.Sub(m.start)
}

func (m *metricsSegment) logStart() {
	m.total.start.Inc(1)
}

func (m *metricsSegment) logFinish(status string, elapsed time.Duration, success bool) {
	m.sync.Lock()
	defer m.sync.Unlock()

	if _, ok := m.status[status]; !ok {
		m.status[status] = m.def.newMetricsItem()
	}

	m.total.finish.Update(elapsed)
	m.status[status].finish.Update(elapsed)

	if success {
		m.total.success.Inc(1)
		m.status[status].success.Inc(1)
	} else {
		m.total.fail.Inc(1)
		m.status[status].fail.Inc(1)
	}
}

type metricsItem struct {
	start   metrics.Counter
	finish  metrics.Timer
	success metrics.Counter
	fail    metrics.Counter
}

type childSegment struct {
	root     *metricsDef
	segments map[int]*metricsSegment
	key      string
}

func (c *childSegment) addSegment(segment int, hit HitSegment) {
	if _, ok := c.segments[segment]; !ok {

		var seg = c.root.newMetricsSegment(hit)
		c.segments[segment] = seg
	}
}

func (c *childSegment) getSegment(segment int) *metricsSegment {
	if seg, ok := c.segments[segment]; ok {
		return seg
	}
	return nil
}
