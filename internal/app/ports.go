package app

/*
#include <libproc.h>
#include <sys/socket.h>
#include <sys/sysctl.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <pwd.h>
#include <unistd.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>

static int ports_list_pids(int *buf, int bufsize) {
	return proc_listpids(PROC_ALL_PIDS, 0, buf, bufsize);
}

static int ports_list_fds(int pid, struct proc_fdinfo *buf, int bufsize) {
	return proc_pidinfo(pid, PROC_PIDLISTFDS, 0, buf, bufsize);
}

static int ports_socket_info(int pid, int fd, struct socket_fdinfo *info) {
	return proc_pidfdinfo(pid, fd, PROC_PIDFDSOCKETINFO, info, (int)sizeof(*info));
}

static int ports_pid_path(int pid, char *buf, int bufsize) {
	return proc_pidpath(pid, buf, (uint32_t)bufsize);
}

static int ports_pid_uid(int pid, uid_t *uid_out) {
	struct kinfo_proc kp;
	int mib[4] = {CTL_KERN, KERN_PROC, KERN_PROC_PID, pid};
	size_t len = sizeof(kp);
	if (sysctl(mib, 4, &kp, &len, NULL, 0) != 0 || len == 0) {
		return -1;
	}
	*uid_out = kp.kp_eproc.e_ucred.cr_uid;
	return 0;
}

static uint16_t ports_ntohs(uint16_t v) {
	return ntohs(v);
}

typedef struct {
	int family;
	int protocol;
	int kind;
	int tcp_state;
	int local_port;
	int foreign_port;
	int vflag;
	unsigned char laddr[16];
	int ok;
} ports_sock_parsed;

static ports_sock_parsed ports_parse_socket(struct socket_fdinfo *info) {
	ports_sock_parsed out;
	memset(&out, 0, sizeof(out));
	out.family = info->psi.soi_family;
	out.protocol = info->psi.soi_protocol;
	out.kind = info->psi.soi_kind;
	out.ok = 0;

	struct in_sockinfo *in = NULL;
	if (out.kind == SOCKINFO_TCP) {
		in = &info->psi.soi_proto.pri_tcp.tcpsi_ini;
		out.tcp_state = info->psi.soi_proto.pri_tcp.tcpsi_state;
	} else if (out.kind == SOCKINFO_IN) {
		in = &info->psi.soi_proto.pri_in;
		out.tcp_state = -1;
	} else {
		return out;
	}
	if (in == NULL) {
		return out;
	}
	out.local_port = (int)ntohs((uint16_t)in->insi_lport);
	out.foreign_port = (int)ntohs((uint16_t)in->insi_fport);
	out.vflag = (int)in->insi_vflag;
	if (out.vflag & INI_IPV4) {
		memcpy(out.laddr, &in->insi_laddr.ina_46.i46a_addr4, 4);
	} else if (out.vflag & INI_IPV6) {
		memcpy(out.laddr, &in->insi_laddr.ina_6, 16);
	} else {
		memcpy(out.laddr, &in->insi_laddr.ina_46.i46a_addr4, 4);
	}
	out.ok = 1;
	return out;
}
*/
import "C"
import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/mactop/v2/internal/i18n"
)

type PortMetrics struct {
	Port        int
	Protocol    string
	Bind        string
	PID         int
	User        string
	Command     string
	Established int
	External    bool
}

type portIdentity struct {
	PID      int
	Protocol string
	Port     int
	Bind     string
}

type establishedKey struct {
	PID      int
	Protocol string
	Port     int
}

var (
	lastPorts             []PortMetrics
	filteredPorts         []PortMetrics
	portsExternalOnly     bool
	portColumns           = []string{"PORT", "PROTO", "BIND", "PID", "USER", "ESTAB", "CMD"}
	portSelectedColumn    = 0
	portSortReverse       = false
	portsCacheMutex       sync.Mutex
	portsProcessNameCache = make(map[int]string)
	portsUserCache        = make(map[int]string)
)

func isPortsLayoutActive() bool {
	return currentConfig.DefaultLayout == LayoutPorts
}

type portListener struct {
	id   portIdentity
	user string
	cmd  string
}

func listAllPIDs() ([]C.int, error) {
	intSize := int(unsafe.Sizeof(C.int(0)))
	needed := C.ports_list_pids(nil, 0)
	if needed <= 0 {
		return nil, fmt.Errorf("proc_listpids size failed")
	}
	pidBuf := make([]C.int, int(needed)/intSize+32)
	n := C.ports_list_pids((*C.int)(unsafe.Pointer(&pidBuf[0])), C.int(len(pidBuf)*intSize))
	if n <= 0 {
		return nil, fmt.Errorf("proc_listpids failed")
	}
	pidCount := int(n) / intSize
	if pidCount > len(pidBuf) {
		pidCount = len(pidBuf)
	}
	return pidBuf[:pidCount], nil
}

func listProcessFDs(pid int, fdScratch []C.struct_proc_fdinfo) ([]C.struct_proc_fdinfo, []C.struct_proc_fdinfo) {
	fdInfoSize := int(C.sizeof_struct_proc_fdinfo)
	fdBytes := C.ports_list_fds(C.int(pid), nil, 0)
	if fdBytes <= 0 {
		return nil, fdScratch
	}
	fdCount := int(fdBytes) / fdInfoSize
	if fdCount <= 0 {
		return nil, fdScratch
	}
	if fdCount > len(fdScratch) {
		fdScratch = make([]C.struct_proc_fdinfo, fdCount+32)
	}
	got := C.ports_list_fds(C.int(pid), &fdScratch[0], C.int(fdCount*fdInfoSize))
	if got <= 0 {
		return nil, fdScratch
	}
	actualFDCount := int(got) / fdInfoSize
	if actualFDCount > len(fdScratch) {
		actualFDCount = len(fdScratch)
	}
	return fdScratch[:actualFDCount], fdScratch
}

func resolvePortProcessMeta(pid int, pathBuf *[C.PROC_PIDPATHINFO_MAXSIZE]C.char) (cmd, user string) {
	cmd = portsProcessNameCache[pid]
	if cmd == "" {
		if C.ports_pid_path(C.int(pid), &pathBuf[0], C.int(len(pathBuf))) > 0 {
			full := C.GoString(&pathBuf[0])
			if idx := strings.LastIndex(full, "/"); idx >= 0 {
				cmd = full[idx+1:]
			} else {
				cmd = full
			}
		}
		if cmd == "" {
			cmd = "?"
		}
		portsProcessNameCache[pid] = cmd
	}
	user = portsUserCache[pid]
	if user == "" {
		var uid C.uid_t
		if C.ports_pid_uid(C.int(pid), &uid) == 0 {
			user = getUsername(uint32(uid))
		} else {
			user = "?"
		}
		portsUserCache[pid] = user
	}
	return cmd, user
}

func collectPortsForPID(pid int, fdScratch []C.struct_proc_fdinfo, pathBuf *[C.PROC_PIDPATHINFO_MAXSIZE]C.char, sock *C.struct_socket_fdinfo, listeners map[portIdentity]portListener, established map[establishedKey]int) []C.struct_proc_fdinfo {
	fds, fdScratch := listProcessFDs(pid, fdScratch)
	if len(fds) == 0 {
		return fdScratch
	}

	cmd := portsProcessNameCache[pid]
	user := portsUserCache[pid]
	resolvedMeta := false

	for j := range fds {
		if fds[j].proc_fdtype != C.PROX_FDTYPE_SOCKET {
			continue
		}
		if C.ports_socket_info(C.int(pid), C.int(fds[j].proc_fd), sock) <= 0 {
			continue
		}
		proto, port, bind, isListen, isEstab := parseSocketParsed(C.ports_parse_socket(sock))
		if port == 0 || proto == "" || (!isListen && !isEstab) {
			continue
		}
		if !resolvedMeta {
			cmd, user = resolvePortProcessMeta(pid, pathBuf)
			resolvedMeta = true
		}
		if isEstab {
			established[establishedKey{PID: pid, Protocol: proto, Port: port}]++
			continue
		}
		id := portIdentity{PID: pid, Protocol: proto, Port: port, Bind: bind}
		if _, ok := listeners[id]; !ok {
			listeners[id] = portListener{id: id, user: user, cmd: cmd}
		}
	}
	return fdScratch
}

func collectListeningPorts() ([]PortMetrics, error) {
	pids, err := listAllPIDs()
	if err != nil {
		return nil, err
	}

	listeners := make(map[portIdentity]portListener)
	established := make(map[establishedKey]int)
	fdScratch := make([]C.struct_proc_fdinfo, 256)
	var sock C.struct_socket_fdinfo
	var pathBuf [C.PROC_PIDPATHINFO_MAXSIZE]C.char

	portsCacheMutex.Lock()
	defer portsCacheMutex.Unlock()

	for _, rawPID := range pids {
		pid := int(rawPID)
		if pid <= 0 {
			continue
		}
		fdScratch = collectPortsForPID(pid, fdScratch, &pathBuf, &sock, listeners, established)
	}

	ports := make([]PortMetrics, 0, len(listeners))
	for id, l := range listeners {
		ports = append(ports, PortMetrics{
			Port:        id.Port,
			Protocol:    id.Protocol,
			Bind:        id.Bind,
			PID:         id.PID,
			User:        l.user,
			Command:     l.cmd,
			Established: established[establishedKey{PID: id.PID, Protocol: id.Protocol, Port: id.Port}],
			External:    isExternalBind(id.Bind),
		})
	}
	return ports, nil
}

func parseSocketParsed(parsed C.ports_sock_parsed) (proto string, port int, bind string, isListen bool, isEstab bool) {
	if parsed.ok == 0 {
		return "", 0, "", false, false
	}
	family := int(parsed.family)
	if family != C.AF_INET && family != C.AF_INET6 {
		return "", 0, "", false, false
	}

	kind := int(parsed.kind)
	protocol := int(parsed.protocol)
	switch kind {
	case C.SOCKINFO_TCP:
		proto = "TCP"
	case C.SOCKINFO_IN:
		switch protocol {
		case C.IPPROTO_UDP:
			proto = "UDP"
		case C.IPPROTO_TCP:
			proto = "TCP"
		default:
			return "", 0, "", false, false
		}
	default:
		return "", 0, "", false, false
	}

	port = int(parsed.local_port)
	if port == 0 {
		return "", 0, "", false, false
	}
	foreignPort := int(parsed.foreign_port)
	bind = formatParsedBind(int(parsed.vflag), parsed.laddr)

	switch proto {
	case "TCP":
		if int(parsed.tcp_state) == C.TSI_S_LISTEN {
			isListen = true
		} else if int(parsed.tcp_state) == C.TSI_S_ESTABLISHED {
			isEstab = true
		}
	case "UDP":
		if foreignPort == 0 {
			isListen = true
		} else {
			isEstab = true
		}
	}
	return proto, port, bind, isListen, isEstab
}

func formatParsedBind(vflag int, laddr [16]C.uchar) string {
	if vflag&C.INI_IPV4 != 0 || (vflag&C.INI_IPV6 == 0) {
		ip := net.IPv4(byte(laddr[0]), byte(laddr[1]), byte(laddr[2]), byte(laddr[3]))
		if ip.Equal(net.IPv4zero) {
			if vflag&C.INI_IPV6 != 0 {
				// fall through to IPv6
			} else {
				return "*"
			}
		} else if vflag&C.INI_IPV6 == 0 || vflag&C.INI_IPV4 != 0 {
			if !ip.Equal(net.IPv4zero) {
				return ip.String()
			}
		}
	}
	if vflag&C.INI_IPV6 != 0 {
		b := make([]byte, 16)
		for i := 0; i < 16; i++ {
			b[i] = byte(laddr[i])
		}
		ip := net.IP(b)
		if ip.Equal(net.IPv6zero) {
			return "[::]"
		}
		return "[" + ip.String() + "]"
	}
	ip := net.IPv4(byte(laddr[0]), byte(laddr[1]), byte(laddr[2]), byte(laddr[3]))
	if ip.Equal(net.IPv4zero) {
		return "*"
	}
	return ip.String()
}

func isExternalBind(bind string) bool {
	switch bind {
	case "127.0.0.1", "::1", "[::1]", "localhost":
		return false
	case "*", "[::]", "0.0.0.0":
		return true
	}
	host := bind
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

func sortPorts(ports []PortMetrics) {
	cols := portColumns
	col := portSelectedColumn
	if col < 0 || col >= len(cols) {
		col = 0
	}
	sort.SliceStable(ports, func(i, j int) bool {
		var less bool
		var equal bool
		switch cols[col] {
		case "PORT":
			less = ports[i].Port < ports[j].Port
			equal = ports[i].Port == ports[j].Port
		case "PROTO":
			less = ports[i].Protocol < ports[j].Protocol
			equal = ports[i].Protocol == ports[j].Protocol
		case "BIND":
			b1, b2 := strings.ToLower(ports[i].Bind), strings.ToLower(ports[j].Bind)
			less = b1 < b2
			equal = b1 == b2
		case "PID":
			less = ports[i].PID < ports[j].PID
			equal = ports[i].PID == ports[j].PID
		case "USER":
			u1, u2 := strings.ToLower(ports[i].User), strings.ToLower(ports[j].User)
			less = u1 < u2
			equal = u1 == u2
		case "ESTAB":
			less = ports[i].Established > ports[j].Established
			equal = ports[i].Established == ports[j].Established
		case "CMD":
			c1, c2 := strings.ToLower(ports[i].Command), strings.ToLower(ports[j].Command)
			less = c1 < c2
			equal = c1 == c2
		default:
			less = ports[i].Port < ports[j].Port
			equal = ports[i].Port == ports[j].Port
		}
		if equal {
			if ports[i].Port != ports[j].Port {
				return ports[i].Port < ports[j].Port
			}
			if ports[i].Protocol != ports[j].Protocol {
				return ports[i].Protocol < ports[j].Protocol
			}
			return ports[i].PID < ports[j].PID
		}
		if portSortReverse {
			return !less
		}
		return less
	})
}

func calculatePortMaxWidths(availableWidth int) map[string]int {
	maxWidths := map[string]int{
		"PORT":  5,
		"PROTO": 5,
		"BIND":  15,
		"PID":   5,
		"USER":  8,
		"ESTAB": 5,
		"CMD":   15,
	}
	usedWidth := 0
	for col, width := range maxWidths {
		if col != "CMD" {
			usedWidth += width + 1
		}
	}
	cmdWidth := availableWidth - usedWidth
	if cmdWidth < 5 {
		cmdWidth = 5
	}
	maxWidths["CMD"] = cmdWidth
	return maxWidths
}

func buildPortHeader(maxWidths map[string]int, themeColorStr, selectedHeaderFg string) string {
	header := ""
	for i, col := range portColumns {
		width := maxWidths[col]
		arrow := ""
		if i == portSelectedColumn {
			arrow = "↓"
			if portSortReverse {
				arrow = "↑"
			}
		}
		colWithArrow := i18n.T("Port_"+col) + arrow
		w := runewidth.StringWidth(colWithArrow)
		padding := width - w
		if padding < 0 {
			padding = 0
		}
		colText := ""
		switch col {
		case "BIND", "USER", "CMD", "PROTO":
			colText = colWithArrow + strings.Repeat(" ", padding)
		default:
			colText = strings.Repeat(" ", padding) + colWithArrow
		}
		header += fmt.Sprintf("[%s](fg:%s,bg:%s)", colText, selectedHeaderFg, themeColorStr)
		if i < len(portColumns)-1 {
			header += fmt.Sprintf("[%s](fg:%s,bg:%s)", "|", selectedHeaderFg, themeColorStr)
		}
	}
	return header
}

func buildPortRows(ports []PortMetrics, maxWidths map[string]int) []string {
	items := make([]string, len(ports))
	for i, p := range ports {
		username := runewidth.Truncate(p.User, maxWidths["USER"], "...")
		bind := runewidth.Truncate(p.Bind, maxWidths["BIND"], "...")
		line := fmt.Sprintf("%*d %-*s %-*s %*d %-*s %*d %-s",
			maxWidths["PORT"], p.Port,
			maxWidths["PROTO"], p.Protocol,
			maxWidths["BIND"], bind,
			maxWidths["PID"], p.PID,
			maxWidths["USER"], username,
			maxWidths["ESTAB"], p.Established,
			runewidth.Truncate(p.Command, maxWidths["CMD"], "..."),
		)
		if i == processList.SelectedRow-1 {
			items[i] = line
			continue
		}
		if p.External {
			items[i] = fmt.Sprintf("[%s](fg:red)", line)
			continue
		}
		if currentUser != "" && currentUser != "root" && p.User != currentUser {
			color := GetProcessTextColor(false)
			items[i] = fmt.Sprintf("[%s](fg:%s)", line, color)
		} else {
			color := GetProcessTextColor(true)
			items[i] = fmt.Sprintf("[%s](fg:%s)", line, color)
		}
	}
	return items
}

func getPortsListTitle() (string, ui.Style) {
	var titleColor ui.Color = ui.ColorClear
	if currentConfig.CustomTheme != nil && currentConfig.CustomTheme.ProcessList != "" {
		if color, err := ParseHexColor(currentConfig.CustomTheme.ProcessList); err == nil {
			titleColor = color
		}
	}
	if titleColor == ui.ColorClear {
		titleColor = GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode)
	}

	if killPending {
		return fmt.Sprintf(i18n.T("TUI_PortsListKill"), killPID), ui.NewStyle(ui.ColorRed, CurrentBgColor, ui.ModifierBold)
	} else if searchMode || searchText != "" {
		return fmt.Sprintf(i18n.T("TUI_PortsListSearch"), searchText), ui.NewStyle(titleColor, CurrentBgColor, ui.ModifierBold)
	} else if isFrozen {
		return i18n.T("TUI_PortsListFrozen"), ui.NewStyle(titleColor, CurrentBgColor, ui.ModifierBold)
	} else if portsExternalOnly {
		return i18n.T("TUI_PortsListExternal"), ui.NewStyle(titleColor, CurrentBgColor, ui.ModifierBold)
	}
	return i18n.T("TUI_PortsListFull"), ui.NewStyle(titleColor, CurrentBgColor)
}

func filterPorts(ports []PortMetrics) []PortMetrics {
	out := ports
	if portsExternalOnly {
		filtered := make([]PortMetrics, 0, len(out))
		for _, p := range out {
			if p.External {
				filtered = append(filtered, p)
			}
		}
		out = filtered
	}
	if searchText != "" {
		lower := strings.ToLower(searchText)
		filtered := make([]PortMetrics, 0, len(out))
		for _, p := range out {
			if portMatchesSearch(p, lower) {
				filtered = append(filtered, p)
			}
		}
		out = filtered
	}
	return out
}

func portMatchesSearch(p PortMetrics, lowerText string) bool {
	if strings.Contains(strings.ToLower(p.Command), lowerText) {
		return true
	}
	if strings.Contains(strings.ToLower(p.User), lowerText) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Bind), lowerText) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Protocol), lowerText) {
		return true
	}
	if strings.Contains(fmt.Sprintf("%d", p.Port), lowerText) {
		return true
	}
	if strings.Contains(fmt.Sprintf("%d", p.PID), lowerText) {
		return true
	}
	return false
}

func refreshFilteredPorts() {
	if searchText == "" && !portsExternalOnly {
		filteredPorts = nil
		return
	}
	filteredPorts = filterPorts(lastPorts)
}

func updateFilteredPorts() {
	refreshFilteredPorts()
	if (searchText != "" || portsExternalOnly) && len(filteredPorts) > 0 {
		processList.SelectedRow = 1
	} else if searchText != "" || portsExternalOnly {
		processList.SelectedRow = 0
	}
}

func updatePortsList() {
	if processList == nil {
		return
	}
	ports := lastPorts
	if searchText != "" || portsExternalOnly {
		if filteredPorts == nil {
			ports = filterPorts(lastPorts)
			filteredPorts = ports
		} else {
			ports = filteredPorts
		}
	}

	sortPorts(ports)

	availableWidth := processList.Inner.Dx()
	if availableWidth <= 0 {
		termWidth, _ := GetCachedTerminalDimensions()
		availableWidth = termWidth - 2
	}
	if availableWidth < 1 {
		availableWidth = 80
	}
	maxWidths := calculatePortMaxWidths(availableWidth)
	themeColorStr, selectedHeaderFg := resolveProcessThemeColor()
	header := buildPortHeader(maxWidths, themeColorStr, selectedHeaderFg)
	rows := buildPortRows(ports, maxWidths)
	items := make([]string, 0, len(rows)+1)
	items = append(items, header)
	items = append(items, rows...)

	if processList.SelectedRow >= len(items) {
		if len(items) > 1 {
			processList.SelectedRow = len(items) - 1
		} else {
			processList.SelectedRow = 0
		}
	}
	if processList.SelectedRow == 0 && len(items) > 1 {
		processList.SelectedRow = 1
	}

	processList.Title, processList.TitleStyle = getPortsListTitle()
	processList.Rows = items
}

func togglePortsExternalFilter() {
	portsExternalOnly = !portsExternalOnly
	updateFilteredPorts()
	updatePortsList()
}

func portsSummary(ports []PortMetrics) (total, external, tcp, udp int) {
	total = len(ports)
	for _, p := range ports {
		if p.External {
			external++
		}
		switch p.Protocol {
		case "TCP":
			tcp++
		case "UDP":
			udp++
		}
	}
	return total, external, tcp, udp
}

func currentViewPorts() []PortMetrics {
	if searchText != "" || portsExternalOnly {
		if filteredPorts == nil {
			return filterPorts(lastPorts)
		}
		return filteredPorts
	}
	return lastPorts
}
