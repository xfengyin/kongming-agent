package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics 提供懒注册的 Prometheus 指标工厂。
// 首次按 (name, labelKeys) 调用会创建并注册到 registry；后续调用复用同一 collector。
// counter / gauge 使用按 name 注册；histogram 会自动追加 "_seconds" 后缀（Prometheus 惯例）。
type Metrics struct {
	registry         *prometheus.Registry
	mu               sync.Mutex // 保护以下 map 的并发安全
	counterFactories map[string]*prometheus.CounterVec
	histFactories    map[string]*prometheus.HistogramVec
	gaugeFactories   map[string]*prometheus.GaugeVec
}

// newMetrics 构造一个懒注册 Metrics，注入自定义 registry（避免污染全局 default registry）。
func newMetrics(reg *prometheus.Registry) *Metrics {
	return &Metrics{
		registry:         reg,
		counterFactories: make(map[string]*prometheus.CounterVec),
		histFactories:    make(map[string]*prometheus.HistogramVec),
		gaugeFactories:   make(map[string]*prometheus.GaugeVec),
	}
}

// IncCounter 按 (name, labels) 累加 1，首次出现则创建并注册 CounterVec。
func (m *Metrics) IncCounter(name string, labels map[string]string) {
	cv, err := m.getOrCreateCounter(name, labelKeys(labels))
	if err != nil {
		return
	}
	cv.With(labels).Inc()
}

// ObserveHistogram 按 (name, labels) 记录一个观测值；指标名自动加 "_seconds" 后缀。
func (m *Metrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	full := name + "_seconds"
	hv, err := m.getOrCreateHistogram(full, labelKeys(labels))
	if err != nil {
		return
	}
	hv.With(labels).Observe(value)
}

// SetGauge 按 (name, labels) 设置当前值，首次出现则创建并注册 GaugeVec。
func (m *Metrics) SetGauge(name string, value float64, labels map[string]string) {
	gv, err := m.getOrCreateGauge(name, labelKeys(labels))
	if err != nil {
		return
	}
	gv.With(labels).Set(value)
}

// getOrCreateCounter 加锁查找/创建 CounterVec；已存在则复用。
// 同一 name 第二次以不同 labelKeys 调用时不能修改已注册的 labelKeys（Prom 限制），
// 因此若不一致则返回错误——调用方需保证同一 name 下 labelKeys 稳定。
func (m *Metrics) getOrCreateCounter(name string, keys []string) (*prometheus.CounterVec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cv, ok := m.counterFactories[name]; ok {
		return cv, nil
	}
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name}, keys)
	if err := m.registry.Register(cv); err != nil {
		return nil, err
	}
	m.counterFactories[name] = cv
	return cv, nil
}

func (m *Metrics) getOrCreateHistogram(name string, keys []string) (*prometheus.HistogramVec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hv, ok := m.histFactories[name]; ok {
		return hv, nil
	}
	hv := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: name, Buckets: prometheus.DefBuckets},
		keys,
	)
	if err := m.registry.Register(hv); err != nil {
		return nil, err
	}
	m.histFactories[name] = hv
	return hv, nil
}

func (m *Metrics) getOrCreateGauge(name string, keys []string) (*prometheus.GaugeVec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if gv, ok := m.gaugeFactories[name]; ok {
		return gv, nil
	}
	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, keys)
	if err := m.registry.Register(gv); err != nil {
		return nil, err
	}
	m.gaugeFactories[name] = gv
	return gv, nil
}

// labelKeys 把 map 的 key 抽出来作为 prom 注册时的 label 列表。
// 顺序不固定（map 迭代），但同一 (name, labels) 调用结果稳定即可。
func labelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}
