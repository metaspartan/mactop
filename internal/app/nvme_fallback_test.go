package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestNVMeHIDFallback(t *testing.T) {
	source, err := os.ReadFile("ioreport.m")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	section := func(start, end string) string {
		t.Helper()
		begin := strings.Index(text, start)
		if begin < 0 {
			t.Fatalf("missing source section %q", start)
		}
		finish := strings.Index(text[begin:], end)
		if finish < 0 {
			t.Fatalf("missing source terminator %q", end)
		}
		return text[begin : begin+finish]
	}
	readFixture := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	fragments := []string{
		section("  static int smcTempTick = 0;", "  smcTempTick++;") + "  smcTempTick++;\n",
		"  if (refreshSlowSensors) {\n    readNVMeSMARTTemps();\n  }\n",
		section("  temp_sensor_t hidSensors[64];", "&hidScount, &hidGPUcount);") + "&hidScount, &hidGPUcount);\n",
	}
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("sampling fragment not found: %s", fragment)
		}
	}
	sort.Slice(fragments, func(first, second int) bool {
		return strings.Index(text, fragments[first]) < strings.Index(text, fragments[second])
	})
	program := readFixture("nvme_temperature_stubs.h") +
		section("typedef struct {\n  uint8_t  critical_warning;", "static void readNVMeSMARTTemps(void)") +
		section("static int readHIDCoreTempSensors(", "static float readSocTemperature(") +
		section("static void readNVMeSMARTTemps(void)", "// --- ANE activity fallback") +
		"static void sample_test_temperatures(void) {\n" + strings.Join(fragments, "\n") + "}\n" +
		readFixture("nvme_temperature_main.c")
	directory := t.TempDir()
	nativeSource := filepath.Join(directory, "nvme_fallback.m")
	if err := os.WriteFile(nativeSource, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "nvme_fallback")
	command := exec.Command("clang", nativeSource, "-o", binary, "-framework", "CoreFoundation", "-framework", "IOKit")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native fallback test: %v\n%s", err, output)
	}
	for _, scenario := range []string{"empty", "enumeration-error", "smart-to-hid", "both-unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			if output, err := exec.Command(binary, scenario).CombinedOutput(); err != nil {
				t.Fatalf("native fallback test: %v\n%s", err, output)
			}
		})
	}
}
