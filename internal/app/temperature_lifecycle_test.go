package app

import (
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusTemperatureLifecycle(t *testing.T) {
	tempSensorGauge.Reset()
	previousTempSensors = map[string]prometheus.Labels{}
	t.Cleanup(func() { tempSensorGauge.Reset(); previousTempSensors = map[string]prometheus.Labels{} })
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
