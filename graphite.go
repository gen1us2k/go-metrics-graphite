package graphite

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"bytes"
	"github.com/rcrowley/go-metrics"
)

// GraphiteConfig provides a container with configuration parameters for
// the Graphite exporter
type GraphiteConfig struct {
	Addr          string           // Network address to connect to
	Registry      metrics.Registry // Registry to be exported
	FlushInterval time.Duration    // Flush interval
	DurationUnit  time.Duration    // Time conversion unit for durations
	Prefix        string           // Prefix to be prepended to metric names
	Percentiles   []float64        // Percentiles to export from timers and histograms
}

// Graphite is a blocking exporter function which reports metrics in r
// to a graphite server located at addr, flushing them every d duration
// and prepending metric names with prefix.
func Graphite(r metrics.Registry, d time.Duration, prefix string, addr string) {
	GraphiteWithConfig(GraphiteConfig{
		Addr:          addr,
		Registry:      r,
		FlushInterval: d,
		DurationUnit:  time.Nanosecond,
		Prefix:        prefix,
		Percentiles:   []float64{0.5, 0.75, 0.95, 0.99, 0.999},
	})
}

// GraphiteWithConfig is a blocking exporter function just like Graphite,
// but it takes a GraphiteConfig instead.
func GraphiteWithConfig(c GraphiteConfig) {
	for _ = range time.Tick(c.FlushInterval) {
		if err := graphite(&c); nil != err {
			log.Println(err)
		}
	}
}

// GraphiteOnce performs a single submission to Graphite, returning a
// non-nil error on failed connections. This can be used in a loop
// similar to GraphiteWithConfig for custom error handling.
func GraphiteOnce(c GraphiteConfig) error {
	return graphite(&c)
}

func graphite(c *GraphiteConfig) error {
	now := time.Now().Unix()
	du := float64(c.DurationUnit)
	conn, err := net.DialTimeout("tcp", c.Addr, 5*time.Second)
	if nil != err {
		return err
	}
	defer conn.Close()
	buf := bytes.NewBufferString("")
	c.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			buf.WriteString(fmt.Sprintf("%s.%s %d %d\n", c.Prefix, name, metric.Count(), now))
		case metrics.Gauge:
			buf.WriteString(fmt.Sprintf("%s.%s %d %d\n", c.Prefix, name, metric.Value(), now))
		case metrics.GaugeFloat64:
			buf.WriteString(fmt.Sprintf("%s.%s %f %d\n", c.Prefix, name, metric.Value(), now))
		case metrics.Histogram:
			h := metric.Snapshot()
			ps := h.Percentiles(c.Percentiles)
			buf.WriteString(fmt.Sprintf("%s.%s.count %d %d\n", c.Prefix, name, h.Count(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.min %d %d\n", c.Prefix, name, h.Min(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.max %d %d\n", c.Prefix, name, h.Max(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.mean %.2f %d\n", c.Prefix, name, h.Mean(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.std-dev %.2f %d\n", c.Prefix, name, h.StdDev(), now))
			for psIdx, psKey := range c.Percentiles {
				key := strings.Replace(strconv.FormatFloat(psKey*100.0, 'f', -1, 64), ".", "", 1)
				buf.WriteString(fmt.Sprintf("%s.%s.%s-precentile %.2f %d\n", c.Prefix, name, key, ps[psIdx], now))
			}
		case metrics.Meter:
			m := metric.Snapshot()
			buf.WriteString(fmt.Sprintf("%s.%s.count %d %d\n", c.Prefix, name, m.Count(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.one-minute %.2f %d\n", c.Prefix, name, m.Rate1(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.five-minute %.2f %d\n", c.Prefix, name, m.Rate5(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.fifteen-minute %.2f %d\n", c.Prefix, name, m.Rate15(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.mean %.2f %d\n", c.Prefix, name, m.RateMean(), now))
		case metrics.Timer:
			t := metric.Snapshot()
			ps := t.Percentiles(c.Percentiles)
			buf.WriteString(fmt.Sprintf("%s.%s.count %d %d\n", c.Prefix, name, t.Count(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.min %d %d\n", c.Prefix, name, t.Min()/int64(du), now))
			buf.WriteString(fmt.Sprintf("%s.%s.max %d %d\n", c.Prefix, name, t.Max()/int64(du), now))
			buf.WriteString(fmt.Sprintf("%s.%s.mean %.2f %d\n", c.Prefix, name, t.Mean()/du, now))
			buf.WriteString(fmt.Sprintf("%s.%s.std-dev %.2f %d\n", c.Prefix, name, t.StdDev()/du, now))
			for psIdx, psKey := range c.Percentiles {
				key := strings.Replace(strconv.FormatFloat(psKey*100.0, 'f', -1, 64), ".", "", 1)
				buf.WriteString(fmt.Sprintf("%s.%s.%s-percentile %.2f %d\n", c.Prefix, name, key, ps[psIdx]/du, now))
			}
			buf.WriteString(fmt.Sprintf("%s.%s.one-minute %.2f %d\n", c.Prefix, name, t.Rate1(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.five-minute %.2f %d\n", c.Prefix, name, t.Rate5(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.fifteen-minute %.2f %d\n", c.Prefix, name, t.Rate15(), now))
			buf.WriteString(fmt.Sprintf("%s.%s.mean-rate %.2f %d\n", c.Prefix, name, t.RateMean(), now))
		}

		conn.Write(buf.Bytes())
	})
	return nil
}
