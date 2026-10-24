package app

import (
	"encoding/binary"
	"image"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
)

func TestParseProcessArguments(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		arguments []string
	}{
		{"script and spaces", []string{"/tmp/python", "-m", "example.worker", "a b"}},
		{"empty argument", []string{"python", "", "--flag"}},
		{"unicode", []string{"python", "雪.py"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := binary.NativeEndian.AppendUint32(nil, uint32(len(test.arguments)))
			data = append(data, []byte("/tmp/python\x00\x00\x00")...)
			data = append(data, []byte(strings.Join(test.arguments, "\x00")+"\x00SECRET=not-an-argument\x00")...)
			path, arguments, err := parseProcessArguments(data)
			if err != nil || path != "/tmp/python" || !reflect.DeepEqual(arguments, test.arguments) {
				t.Fatalf("got %q %q %v", path, arguments, err)
			}
		})
	}
	for _, data := range [][]byte{nil, {1, 0, 0, 0}, {255, 255, 255, 255, 0}, {1, 0, 0, 0, 'x', 0, 'a'}} {
		if _, _, err := parseProcessArguments(data); err == nil {
			t.Fatal("accepted malformed arguments")
		}
	}
}

func TestLoadProcessDetails(t *testing.T) {
	t.Parallel()
	lines := strings.Join(loadProcessDetails(os.Getpid()), "\n")
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{path, cwd, "Full command:", "Parent PID:"} {
		if !strings.Contains(lines, expected) {
			t.Fatalf("missing %q in %s", expected, lines)
		}
	}
	if lines := strings.Join(loadProcessDetails(2147483647), "\n"); !strings.Contains(lines, "unavailable") {
		t.Fatal(lines)
	}
}

func TestProcessDetailsWrapping(t *testing.T) {
	t.Parallel()
	for _, width := range []int{1, 8, 60} {
		lines := wrapProcessDetails([]string{"/a/very/long/雪/path [literal](fg:red)\x1b\tend"}, width)
		for _, line := range lines {
			if runewidth.StringWidth(line) > width || strings.ContainsAny(line, "\x1b\t") {
				t.Fatalf("unsafe line %q", line)
			}
		}
	}
	if got := quoteProcessArgument("a'b c"); got != "'a'\"'\"'b c'" {
		t.Fatal(got)
	}
}

func TestProcessDetailsScrollAndResize(t *testing.T) {
	original := processDetails
	t.Cleanup(func() { processDetails = original })
	view := &processDetailsView{Block: ui.NewBlock(), lines: []string{strings.Repeat("x", 500), "last line"}}
	processDetails = view
	view.SetRect(0, 0, 30, 10)
	view.Draw(ui.NewBuffer(image.Rect(0, 0, 30, 10)))
	if !handleProcessDetailsKey("<PageDown>") || view.offset != view.pageSize {
		t.Fatal("page down failed")
	}
	handleProcessDetailsKey("<End>")
	view.Draw(ui.NewBuffer(image.Rect(0, 0, 30, 10)))
	if view.offset == 0 {
		t.Fatal("could not reach long command end")
	}
	view.SetRect(0, 0, 100, 30)
	view.Draw(ui.NewBuffer(image.Rect(0, 0, 100, 30)))
	if view.offset != 0 {
		t.Fatal("resize did not clamp scroll")
	}
	view.SetRect(0, 0, 2, 2)
	view.Draw(ui.NewBuffer(image.Rect(0, 0, 2, 2)))
	handleProcessDetailsKey("<Escape>")
	if processDetails != nil {
		t.Fatal("escape did not close popup")
	}
}

func TestProcessDetailsResponsiveLayout(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		width, height int
		docked        bool
	}{
		{80, 24, false}, {149, 40, false}, {180, 23, false}, {150, 24, true}, {220, 60, true},
	} {
		if got := processDetailsDocked(test.width, test.height); got != test.docked {
			t.Fatalf("%dx%d: docked=%v", test.width, test.height, got)
		}
		if !test.docked {
			continue
		}
		table, details := processDetailsRects(test.width, test.height)
		if table.Overlaps(details) || table.Dx() < 90 || details.Dx() < 45 {
			t.Fatalf("bad split: %v %v", table, details)
		}
		screen := image.Rect(0, 0, test.width, test.height)
		if !table.In(screen) || !details.In(screen) {
			t.Fatal("panel outside terminal")
		}
	}
}

func TestProcessDetailsFocusRouting(t *testing.T) {
	original := processDetails
	t.Cleanup(func() { processDetails = original })
	for _, test := range []struct {
		key              string
		focused, handled bool
		offset           int
	}{
		{"<Down>", false, false, 0}, {"<Up>", false, false, 0},
		{"<Left>", false, false, 0}, {"<Right>", true, false, 0}, {"<Enter>", true, false, 0},
		{"/", false, false, 0}, {"q", true, false, 0},
		{"<Down>", true, true, 1}, {"<PageDown>", true, true, 5},
	} {
		processDetails = &processDetailsView{focused: test.focused, pageSize: 5}
		if handled := handleDockedDetailsKey(test.key); handled != test.handled || processDetails.offset != test.offset {
			t.Fatalf("key=%s focused=%v handled=%v offset=%d", test.key, test.focused, handled, processDetails.offset)
		}
	}
	processDetails = &processDetailsView{}
	if !handleDockedDetailsKey("<Tab>") || !processDetails.focused {
		t.Fatal("Tab did not focus details")
	}
	if !handleDockedDetailsKey("<Tab>") || processDetails.focused {
		t.Fatal("Tab did not restore list focus")
	}
	if !handleDockedDetailsKey("<Escape>") || processDetails != nil {
		t.Fatal("Escape did not close details")
	}
}

func TestProcessDetailsSelectionSurvivesRefresh(t *testing.T) {
	originalList, originalProcesses, originalFiltered := processList, lastProcesses, filteredProcesses
	originalSearch, originalLayout := searchText, currentConfig.DefaultLayout
	t.Cleanup(func() {
		processList, lastProcesses, filteredProcesses = originalList, originalProcesses, originalFiltered
		searchText, currentConfig.DefaultLayout = originalSearch, originalLayout
	})
	currentConfig.DefaultLayout = LayoutDefault
	processList = w.NewList()
	processList.Rows = []string{"header", "first", "second"}
	processList.SelectedRow = 1
	for _, filtered := range []bool{false, true} {
		searchText = ""
		if filtered {
			searchText = "worker"
		}
		lastProcesses = []ProcessMetrics{{PID: 21}, {PID: 42}}
		filteredProcesses = append([]ProcessMetrics(nil), lastProcesses...)
		processList.SelectedRow = 1
		selectedPID := selectedDetailsPID()
		lastProcesses = []ProcessMetrics{{PID: 42}, {PID: 21}}
		filteredProcesses = append([]ProcessMetrics(nil), lastProcesses...)
		restoreProcessDetailsSelection(selectedPID)
		if selectedDetailsPID() != 21 || processList.SelectedRow != 2 {
			t.Fatal("selection changed after refresh")
		}
		lastProcesses, filteredProcesses = nil, nil
		processList.Rows = []string{"header"}
		restoreProcessDetailsSelection(selectedPID)
		if processList.SelectedRow != 0 {
			t.Fatal("exited process left invalid selection")
		}
		processList.Rows = []string{"header", "first", "second"}
	}
}
