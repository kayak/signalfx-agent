package monitors

import (
	"github.com/signalfx/neo-agent/core/config"
)

// This code is somewhat convoluted, but basically it creates two types of mock
// monitors, static and dynamic.  It handles doing basic tracking of whether
// the instances have been configured and how, so that we don't have to pry
// into the internals of the manager.

type Config struct {
	config.MonitorConfig
	MyVar   string
	MySlice []string
}

type DynamicConfig struct {
	config.MonitorConfig `acceptsEndpoints:"true"`

	Host string `yaml:"host" validate:"required"`
	Port uint16 `yaml:"port" validate:"required"`
	Name string `yaml:"name"`

	MyVar string
}

type MockMonitor interface {
	SetConfigHook(func(MockMonitor))
	AddShutdownHook(fn func())
	Type() string
}

type _MockMonitor struct {
	MType         string
	shutdownHooks []func()
	configHook    func(MockMonitor)
}

var lastID = 0

func (m *_MockMonitor) Configure(conf *Config) error {
	m.MType = conf.Type
	m.configHook(m)
	return nil
}

func (m *_MockMonitor) Type() string {
	return m.MType
}

func (m *_MockMonitor) SetConfigHook(fn func(MockMonitor)) {
	m.configHook = fn
}

func (m *_MockMonitor) AddShutdownHook(fn func()) {
	m.shutdownHooks = append(m.shutdownHooks, fn)
}

func (m *_MockMonitor) Shutdown() {
	for _, hook := range m.shutdownHooks {
		hook()
	}
}

type _MockServiceMonitor struct {
	_MockMonitor
}

func (m *_MockServiceMonitor) Configure(conf *DynamicConfig) error {
	m.MType = conf.Type
	m.configHook(m)
	return nil
}

type Static1 struct{ _MockMonitor }
type Static2 struct{ _MockMonitor }
type Dynamic1 struct{ _MockServiceMonitor }
type Dynamic2 struct{ _MockServiceMonitor }

func RegisterFakeMonitors() func() map[MonitorID]MockMonitor {
	instances := map[MonitorID]MockMonitor{}

	track := func(factory func() interface{}) func(MonitorID) interface{} {
		return func(id MonitorID) interface{} {
			mon := factory().(MockMonitor)
			mon.SetConfigHook(func(mon MockMonitor) {
				instances[id] = mon
			})
			mon.AddShutdownHook(func() {
				delete(instances, id)
			})

			return mon
		}
	}

	Register("static1", track(func() interface{} { return &Static1{} }), &Config{})
	Register("static2", track(func() interface{} { return &Static2{} }), &Config{})
	Register("dynamic1", track(func() interface{} { return &Dynamic1{} }), &DynamicConfig{})
	Register("dynamic2", track(func() interface{} { return &Dynamic2{} }), &DynamicConfig{})

	return func() map[MonitorID]MockMonitor {
		return instances
	}
}

func findMonitorsByType(monitors map[MonitorID]MockMonitor, _type string) []MockMonitor {
	mons := []MockMonitor{}
	for _, m := range monitors {
		if m.Type() == _type {
			mons = append(mons, m)
		}
	}
	return mons
}