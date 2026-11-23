package app

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusTemperatureLifecycle(t *testing.T) {
	tempSensorGauge.Reset()
	tempSensorChildren = map[[2]string]*tempSensorChild{}
	t.Cleanup(func() { tempSensorGauge.Reset(); tempSensorChildren = map[[2]string]*tempSensorChild{} })
	registry := prometheus.NewRegistry()
	registry.MustRegister(tempSensorGauge)
	tests := []struct {
		name    string
		sensors []TempSensor
	}{
		{"initial", []TempSensor{{Key: "Nv00", Name: "Internal", Value: 33}, {Key: "TS0p", Name: "SSD", Value: 40}}},
		{"reused index", []TempSensor{{Key: "Nv00", Name: "External", Value: 43}, {Key: "TS0p", Name: "SSD", Value: 41}}},
		{"removed disk", []TempSensor{{Key: "TS0p", Name: "SSD", Value: 42}}},
		{"all removed", nil},
		{"recovered", []TempSensor{{Key: "Nv00", Name: "Internal", Value: 35}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updatePrometheusSensors(nil, test.sensors)
			families, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]float64{}
			for _, family := range families {
				for _, metric := range family.Metric {
					labels := map[string]string{}
					for _, label := range metric.Label {
						labels[label.GetName()] = label.GetValue()
					}
					got[labels["key"]+"\x00"+labels["name"]] = metric.GetGauge().GetValue()
				}
			}
			want := map[string]float64{}
			for _, sensor := range test.sensors {
				want[sensor.Key+"\x00"+sensor.Name] = sensor.Value
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("temperatures = %#v, want %#v", got, want)
			}
		})
	}
}

func BenchmarkUpdatePrometheusSensors(b *testing.B) {
	sensors := make([]TempSensor, 281) // 281 sensors on an M5 Max Mac Studio
	for i := range sensors {
		sensors[i] = TempSensor{Key: fmt.Sprintf("Tp%02x", i), Name: fmt.Sprintf("Sensor %d", i), Value: 40 + float64(i)}
	}
	tempSensorGauge.Reset()
	tempSensorChildren = map[[2]string]*tempSensorChild{}
	b.Cleanup(func() { tempSensorGauge.Reset(); tempSensorChildren = map[[2]string]*tempSensorChild{} })
	b.ReportAllocs()
	for b.Loop() {
		updatePrometheusSensors(nil, sensors)
	}
}
