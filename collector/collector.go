package collector

import (
	"log"
	"sync"
	"time"

	"github.com/juniorz/edgemax-exporter/api"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ns = "edgemax"

	cpuUsageDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "cpu", "usage_percent"),
		"System CPU usage (percent).", nil, nil,
	)
	memUsageDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "mem", "usage_mb"),
		"System memory usage (megabytes).", nil, nil,
	)
	uptimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "uptime", "seconds_total"),
		"System uptime (seconds).", nil, nil,
	)

	ifaceLabelsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "labels"),
		"Interface labels.", []string{
			"interface", "mac",
		}, nil,
	)

	ifaceUpDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "up"),
		"Interface is UP.", []string{"interface"}, nil,
	)

	ifaceL1UpDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "l1_up"),
		"Interface L1 is UP.", []string{"interface"}, nil,
	)

	ifaceRxPacketsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "rx_packets_total"),
		"Interface received packets.", []string{"interface"}, nil,
	)

	ifaceTxPacketsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "tx_packets_total"),
		"Interface transmitted packets.", []string{"interface"}, nil,
	)

	ifaceRxBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "rx_bytes_total"),
		"Interface received bytes.", []string{"interface"}, nil,
	)

	ifaceTxBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "tx_bytes_total"),
		"Interface transmitted bytes.", []string{"interface"}, nil,
	)

	ifaceRxErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "rx_errors_total"),
		"Interface received packet errors.", []string{"interface"}, nil,
	)

	ifaceTxErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "tx_errors_total"),
		"Interface transmitted packet errors.", []string{"interface"}, nil,
	)

	ifaceRxDroppedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "rx_dropped_total"),
		"Interface received packets dropped.", []string{"interface"}, nil,
	)

	ifaceTxDroppedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "tx_dropped_total"),
		"Interface transmitted packets dropped.", []string{"interface"}, nil,
	)

	ifaceMulticastDesc = prometheus.NewDesc(
		prometheus.BuildFQName(ns, "interface", "multicast_total"),
		"Interface multicast packets.", []string{"interface"}, nil,
	)
)

type collector struct {
	sync.RWMutex

	*api.SystemStat
	interfaceStat map[string]*api.InterfaceStat
}

func New(c *api.Client) prometheus.Collector {
	ret := &collector{
		interfaceStat: make(map[string]*api.InterfaceStat, 5),
	}

	go ret.poolStatsFrom(c)

	return ret
}

func (c *collector) poolStatsFrom(client *api.Client) {
	for {
		err := c.loginAndSubscribe(client)
		log.Printf("error: %s", err)

		log.Print("Reconnecting in 5 seconds...")
		<-time.After(5 * time.Second)
	}
}

func (c *collector) loginAndSubscribe(client *api.Client) error {
	if err := client.Login(); err != nil {
		return err
	}

	log.Printf("Logged in as %s\n", client.Username)
	defer client.Close()

	subs, err := client.Subscribe(
		"interfaces",
		"system-stats",
		// "export",
		// "discover",
		// "pon-stats",
		// "num-routes",
		// "config-change",
		// "users",
	)

	if err != nil {
		return err
	}

	defer subs.Stop()

	for msg := range subs.C {
		c.updateStats(msg)
	}

	return <-subs.Err
}

func (c *collector) updateStats(msg interface{}) {
	c.Lock()
	defer c.Unlock()

	switch m := msg.(type) {
	case *api.SystemStat:
		c.SystemStat = m
	case *api.InterfaceStat:
		c.interfaceStat[m.Name] = m
	default:
		log.Printf("unknown stats: %#v", m)
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cpuUsageDesc
	ch <- memUsageDesc
	ch <- uptimeDesc

	ch <- ifaceLabelsDesc
	ch <- ifaceUpDesc
	ch <- ifaceL1UpDesc
	ch <- ifaceRxPacketsDesc
	ch <- ifaceTxPacketsDesc
	ch <- ifaceRxBytesDesc
	ch <- ifaceTxBytesDesc
	ch <- ifaceRxErrorsDesc
	ch <- ifaceTxErrorsDesc
	ch <- ifaceRxDroppedDesc
	ch <- ifaceTxDroppedDesc
	ch <- ifaceMulticastDesc
}

func (c *collector) collectInterfaceMetrics(ch chan<- prometheus.Metric) {
	defer c.RUnlock()
	c.RLock()

	for _, stat := range c.interfaceStat {
		ch <- prometheus.MustNewConstMetric(
			ifaceLabelsDesc,
			prometheus.GaugeValue,
			float64(1),
			stat.Name, stat.MAC,
		)

		up := float64(0)
		if stat.Up {
			up = 1
		}
		ch <- prometheus.MustNewConstMetric(
			ifaceUpDesc,
			prometheus.GaugeValue,
			up,
			stat.Name,
		)

		l1up := float64(0)
		if stat.L1Up {
			l1up = 1
		}
		ch <- prometheus.MustNewConstMetric(
			ifaceL1UpDesc,
			prometheus.GaugeValue,
			l1up,
			stat.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			ifaceRxPacketsDesc,
			prometheus.CounterValue,
			float64(stat.RxPackets),
			stat.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ifaceTxPacketsDesc,
			prometheus.CounterValue,
			float64(stat.TxPackets),
			stat.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			ifaceRxBytesDesc,
			prometheus.CounterValue,
			float64(stat.RxBytes),
			stat.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ifaceTxBytesDesc,
			prometheus.CounterValue,
			float64(stat.TxBytes),
			stat.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			ifaceRxErrorsDesc,
			prometheus.CounterValue,
			float64(stat.RxErrors),
			stat.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ifaceTxErrorsDesc,
			prometheus.CounterValue,
			float64(stat.TxErrors),
			stat.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			ifaceRxDroppedDesc,
			prometheus.CounterValue,
			float64(stat.RxDropped),
			stat.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ifaceTxDroppedDesc,
			prometheus.CounterValue,
			float64(stat.TxDropped),
			stat.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			ifaceMulticastDesc,
			prometheus.CounterValue,
			float64(stat.Multicast),
			stat.Name,
		)
	}
}

func (c *collector) collectSystemStats(ch chan<- prometheus.Metric) {
	if c.SystemStat == nil {
		return
	}

	defer c.RUnlock()
	c.RLock()

	ch <- prometheus.MustNewConstMetric(
		cpuUsageDesc,
		prometheus.GaugeValue,
		float64(c.SystemStat.CPU),
	)

	ch <- prometheus.MustNewConstMetric(
		memUsageDesc,
		prometheus.GaugeValue,
		float64(c.SystemStat.Mem),
	)

	ch <- prometheus.MustNewConstMetric(
		uptimeDesc,
		prometheus.CounterValue,
		float64(c.SystemStat.Uptime),
	)
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.collectSystemStats(ch)
	c.collectInterfaceMetrics(ch)
}
