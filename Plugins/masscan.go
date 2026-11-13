package Plugins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/killmonday/fscanx/common"
)

var (
	masscanStatusRe     = regexp.MustCompile(`^rate:\s*[0-9.]+-kpps,\s*[0-9.]+%\s*done,.*found=(\d+)`)
	masscanRangeRe      = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3}){3})-(\d{1,3}(?:\.\d{1,3}){3})$`)
	masscanShortRangeRe = regexp.MustCompile(`^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$`)
)

type masscanStatus struct {
	Found int64
	Last  string
	tail  []string
}

func (s *masscanStatus) addTail(line string) {
	if len(s.tail) >= 20 {
		s.tail = s.tail[1:]
	}
	s.tail = append(s.tail, line)
}

func (s *masscanStatus) Tail() string {
	return strings.Join(s.tail, "\n")
}

// masscanCountingReader 统计 masscan stdout 中标准发现行的数量，作为进度 found 的兜底。
type masscanCountingReader struct {
	r     io.Reader
	buf   []byte
	count int64
}

func (c *masscanCountingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.buf = append(c.buf, p[:n]...)
		c.consume(false)
	}
	if err == io.EOF {
		c.consume(true)
	}
	return n, err
}

func (c *masscanCountingReader) consume(final bool) {
	for {
		idx := bytes.IndexByte(c.buf, '\n')
		if idx < 0 {
			break
		}
		line := bytes.TrimSpace(c.buf[:idx])
		if common.RegMasscanRunningText.Match(line) {
			c.count++
		}
		c.buf = c.buf[idx+1:]
	}
	if final {
		line := bytes.TrimSpace(c.buf)
		if len(line) > 0 && common.RegMasscanRunningText.Match(line) {
			c.count++
		}
		c.buf = nil
	}
}

// ScanWithMasscan 启动 masscan，把 stdout 流式送进 --std 扫描管线，stderr 只用于进度和错误尾部。
func ScanWithMasscan(host string) error {
	start := time.Now()

	bin, err := findMasscanBinary(common.MasscanPath, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	targetsFile, targetCount, err := writeMasscanTargetFile(host, common.HostFile)
	if err != nil {
		return err
	}
	defer os.Remove(targetsFile)

	excludeFile, err := writeMasscanExcludeFile(common.NoHosts)
	if err != nil {
		return err
	}
	if excludeFile != "" {
		defer os.Remove(excludeFile)
	}

	ports, err := normalizeMasscanPorts(common.PortsInput, common.NotScanPorts)
	if err != nil {
		return err
	}

	extra, err := parseMasscanExtraArgs(common.MasscanArgs)
	if err != nil {
		return err
	}

	args := []string{"--rate", strconv.Itoa(common.MasscanRate), "-p" + ports, "-iL", targetsFile}
	if excludeFile != "" {
		args = append(args, "--excludefile", excludeFile)
	}
	args = append(args, extra...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("masscan stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return fmt.Errorf("masscan stderr pipe: %w", err)
	}

	common.LogSuccess("[*] masscan: %s (rate=%d, targets=%d)\n", bin, common.MasscanRate, targetCount)
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return fmt.Errorf("start masscan %s: %w", bin, err)
	}

	common.StartProgress()
	defer common.StopProgress()

	var stderrWG sync.WaitGroup
	status := &masscanStatus{}
	stderrWG.Add(1)
	go func() {
		defer stderrWG.Done()
		parseMasscanStderr(stderr, status)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	interrupted := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			close(interrupted)
			common.Println("[*] interrupt received; stopping masscan; in-flight scans will finish (press Ctrl+C again to force quit)")
			cancel()
			select {
			case <-sigCh:
				common.StopProgress()
				_ = os.Remove(targetsFile)
				if excludeFile != "" {
					_ = os.Remove(excludeFile)
				}
				os.Exit(130)
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
	}()

	counted := &masscanCountingReader{r: stdout}
	ScanFromReader(counted, "masscan")

	stderrWG.Wait()
	waitErr := cmd.Wait()
	common.StopProgress()

	found := status.Found
	if counted.count > found {
		found = counted.count
	}

	select {
	case <-interrupted:
		return errors.New("interrupted")
	default:
	}
	if waitErr != nil {
		if tail := status.Tail(); tail != "" {
			return fmt.Errorf("masscan exited with error: %v\n%s", waitErr, tail)
		}
		return fmt.Errorf("masscan exited with error: %w", waitErr)
	}

	common.LogSuccess("[*] masscan done: found=%d, cost=%s\n", found, time.Since(start))
	return nil
}

func parseMasscanStderr(r io.Reader, status *masscanStatus) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	scanner.Split(splitMasscanStatusLine)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if m := masscanStatusRe.FindStringSubmatch(line); m != nil {
			if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				status.Found = n
			}
			status.Last = "[masscan] " + strings.Join(strings.Fields(line), " ")
			common.UpdateProgress(status.Last)
			continue
		}
		if looksLikeMasscanError(line) {
			status.addTail(line)
		}
	}
}

// splitMasscanStatusLine 同时按 \r 和 \n 切分，因为 masscan 的状态行只以 \r 结尾。
func splitMasscanStatusLine(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\r' || b == '\n' {
			return i + 1, bytes.TrimSpace(data[:i]), nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), bytes.TrimSpace(data), nil
	}
	return 0, nil, nil
}

func looksLikeMasscanError(line string) bool {
	lower := strings.ToLower(line)
	return strings.HasPrefix(line, "[-]") ||
		strings.Contains(lower, "error") ||
		strings.Contains(lower, "fail") ||
		strings.Contains(lower, "could not") ||
		strings.Contains(lower, "permission")
}

func writeMasscanTargetFile(host, hostFile string) (string, int, error) {
	var tokens []string
	if strings.TrimSpace(host) != "" {
		tokens = append(tokens, splitMasscanTargetLine(host)...)
	}
	if hostFile != "" {
		lines, err := readMasscanTargetLines(hostFile)
		if err != nil {
			return "", 0, err
		}
		for _, line := range lines {
			tokens = append(tokens, splitMasscanTargetLine(line)...)
		}
	}

	targets, err := normalizeMasscanTargets(tokens)
	if err != nil {
		return "", 0, err
	}
	if len(targets) == 0 {
		return "", 0, errors.New("no valid masscan targets from -h/-hf")
	}

	path, err := writeMasscanTempFile("fscanx-masscan-targets-*.txt", targets)
	if err != nil {
		return "", 0, err
	}
	return path, len(targets), nil
}

func writeMasscanExcludeFile(noHosts string) (string, error) {
	tokens := splitMasscanTargetLine(noHosts)
	if len(tokens) == 0 {
		return "", nil
	}
	excludes, err := normalizeMasscanTargets(tokens)
	if err != nil {
		return "", err
	}
	if len(excludes) == 0 {
		return "", nil
	}
	return writeMasscanTempFile("fscanx-masscan-exclude-*.txt", excludes)
}

func writeMasscanTempFile(pattern string, lines []string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create masscan temp file: %w", err)
	}
	name := f.Name()
	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			_ = f.Close()
			_ = os.Remove(name)
			return "", fmt.Errorf("write masscan temp file: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("close masscan temp file: %w", err)
	}
	return name, nil
}

func readMasscanTargetLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return lines, nil
}

func splitMasscanTargetLine(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if line == "" {
		return nil
	}
	// masscan 的屏幕输出带空格，必须整行保留给后续正则提取 IP。
	if common.RegMasscanRunningText.MatchString(line) || common.RegMasscanOutputText.MatchString(line) {
		return []string{line}
	}
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		// 容忍 "ip:port 额外说明" 这类行，只取第一个 token。
		out = append(out, fields[0])
	}
	return out
}

func normalizeMasscanTargets(tokens []string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string
	for _, token := range tokens {
		value := strings.TrimSpace(token)
		if value == "" {
			continue
		}

		lowerValue := strings.ToLower(value)
		if strings.HasPrefix(lowerValue, "http://") || strings.HasPrefix(lowerValue, "https://") {
			u, err := url.Parse(lowerValue)
			if err != nil {
				return nil, fmt.Errorf("parse target %q: %w", value, err)
			}
			value = u.Hostname()
		}

		if m := common.RegMasscanRunningText.FindStringSubmatch(value); m != nil {
			value = m[2]
		} else if m := common.RegMasscanOutputText.FindStringSubmatch(value); m != nil {
			value = m[2]
		} else if host, ok, err := stripMasscanTargetPort(value); err != nil {
			return nil, err
		} else if ok {
			value = host
		}

		normalized, err := normalizeMasscanTargetToken(value)
		if err != nil {
			return nil, err
		}
		for _, item := range normalized {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out, nil
}

func stripMasscanTargetPort(value string) (string, bool, error) {
	switch strings.Count(value, ":") {
	case 0:
		return value, false, nil
	case 1:
		host, port, _ := strings.Cut(value, ":")
		if _, err := strconv.Atoi(port); err != nil {
			return value, false, nil
		}
		return host, true, nil
	default:
		return "", false, fmt.Errorf("IPv6 target %q is not supported by masscan", value)
	}
}

func normalizeMasscanTargetToken(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "10":
		return []string{"10.0.0.0/8"}, nil
	case "172":
		return []string{"172.16.0.0/12"}, nil
	case "192":
		return []string{"192.168.0.0/16"}, nil
	}

	if ip := net.ParseIP(value); ip != nil {
		if ip.To4() == nil {
			return nil, fmt.Errorf("IPv6 target %q is not supported by masscan", value)
		}
		return []string{value}, nil
	}

	if strings.Contains(value, "/") {
		ip, _, err := net.ParseCIDR(value)
		if err != nil || ip.To4() == nil {
			return nil, fmt.Errorf("invalid CIDR target %q", value)
		}
		return []string{value}, nil
	}

	if m := masscanRangeRe.FindStringSubmatch(value); m != nil {
		startIP := net.ParseIP(m[1])
		endIP := net.ParseIP(m[2])
		if startIP == nil || endIP == nil || bytes.Compare(startIP.To4(), endIP.To4()) > 0 {
			return nil, fmt.Errorf("invalid range target %q", value)
		}
		return []string{value}, nil
	}

	if m := masscanShortRangeRe.FindStringSubmatch(value); m != nil {
		start, err1 := strconv.Atoi(m[2])
		end, err2 := strconv.Atoi(m[3])
		if err1 != nil || err2 != nil || start < 0 || end > 255 || start > end || net.ParseIP(m[1]+"0") == nil {
			return nil, fmt.Errorf("invalid range target %q", value)
		}
		return []string{m[1] + strconv.Itoa(start) + "-" + m[1] + strconv.Itoa(end)}, nil
	}

	ips, err := common.LookupHost(value)
	if err != nil {
		return nil, fmt.Errorf("resolve target %q: %w", value, err)
	}
	seen := make(map[string]struct{})
	var out []string
	for _, addr := range ips {
		if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			out = append(out, addr)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("target %q has no IPv4 address", value)
	}
	return out, nil
}

func normalizeMasscanPorts(raw, exclude string) (string, error) {
	raw = strings.ReplaceAll(raw, "，", ",")
	exclude = strings.ReplaceAll(exclude, "，", ",")

	ports := common.ParsePort(raw)
	if len(ports) == 0 {
		return "", fmt.Errorf("no valid ports in %q", raw)
	}
	excluded := common.ParsePort(exclude)
	if len(excluded) > 0 {
		set := make(map[int]struct{}, len(excluded))
		for _, port := range excluded {
			set[port] = struct{}{}
		}
		filtered := ports[:0]
		for _, port := range ports {
			if _, ok := set[port]; !ok {
				filtered = append(filtered, port)
			}
		}
		ports = filtered
	}
	if len(ports) == 0 {
		return "", errors.New("all ports are excluded by -pn")
	}

	sort.Ints(ports)
	var parts []string
	for i := 0; i < len(ports); i++ {
		j := i
		for j+1 < len(ports) && ports[j+1] == ports[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, fmt.Sprintf("%d-%d", ports[i], ports[j]))
		} else {
			parts = append(parts, strconv.Itoa(ports[i]))
		}
		i = j
	}
	return strings.Join(parts, ","), nil
}

func parseMasscanExtraArgs(raw string) ([]string, error) {
	fields := strings.Fields(raw)
	for _, field := range fields {
		if common.IsMasscanManagedArg(field) {
			return nil, fmt.Errorf("masscan option %q is managed by fscanx; use -mr/-p/-h/-hf/-hn instead", field)
		}
	}
	return fields, nil
}

func findMasscanBinary(userPath, goos, goarch string) (string, error) {
	if strings.TrimSpace(userPath) != "" {
		abs, err := filepath.Abs(userPath)
		if err != nil {
			return "", fmt.Errorf("resolve -mp %q: %w", userPath, err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("masscan path %q: %w", abs, err)
		}
		if fi.IsDir() {
			return "", fmt.Errorf("masscan path %q is a directory", abs)
		}
		return abs, nil
	}

	if path, err := exec.LookPath("masscan"); err == nil {
		return path, nil
	}
	if goos == "windows" {
		if path, err := exec.LookPath("masscan.exe"); err == nil {
			return path, nil
		}
	}

	var dirs []string
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if len(dirs) == 0 || !samePath(dir, dirs[0]) {
			dirs = append(dirs, dir)
		}
	}
	return findMasscanInDirs(dirs, goos, goarch)
}

func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func collectMasscanCandidates(dirs []string, goos, goarch string) []masscanCandidate {
	var candidates []masscanCandidate
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(strings.ToLower(entry.Name()), "masscan") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			score, reason := inspectMasscanBinary(path, goos, goarch)
			candidates = append(candidates, masscanCandidate{path: path, score: score, reason: reason})
		}
	}
	return candidates
}

func rankMasscanCandidates(candidates []masscanCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].path < candidates[j].path
	})
}

func findMasscanInDirs(dirs []string, goos, goarch string) (string, error) {
	candidates := collectMasscanCandidates(dirs, goos, goarch)
	if len(candidates) == 0 {
		return "", fmt.Errorf("masscan not found in PATH or these directories: %s", strings.Join(dirs, ", "))
	}

	rankMasscanCandidates(candidates)

	var failures []string
	for _, candidate := range candidates {
		if candidate.score < 0 {
			failures = append(failures, fmt.Sprintf("%s: %s", candidate.path, candidate.reason))
			continue
		}
		if err := testMasscanBinary(candidate.path); err == nil {
			return candidate.path, nil
		} else {
			failures = append(failures, fmt.Sprintf("%s: %s; %v", candidate.path, candidate.reason, err))
		}
	}
	return "", fmt.Errorf("no runnable masscan binary found:\n%s", strings.Join(failures, "\n"))
}

type masscanCandidate struct {
	path   string
	score  int
	reason string
}

func testMasscanBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == -1073741515 {
			return fmt.Errorf("--version failed: exit code %d (missing DLL, install Npcap?)", exitErr.ExitCode())
		}
		msg := strings.TrimSpace(string(out))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		if msg != "" {
			return fmt.Errorf("--version failed: %v: %s", err, msg)
		}
		return fmt.Errorf("--version failed: %w", err)
	}
	if !strings.Contains(strings.ToLower(string(out)), "masscan") {
		return errors.New("--version output does not look like masscan")
	}
	return nil
}

func inspectMasscanBinary(path, goos, goarch string) (int, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err.Error()
	}
	// 可执行位是宿主文件系统的属性；生产环境中 goos == runtime.GOOS。
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0111 == 0 {
		return -1, "not executable (no execute permission)"
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, err.Error()
	}
	defer f.Close()

	header := make([]byte, 4096)
	n, _ := io.ReadFull(f, header)
	header = header[:n]
	if n < 4 {
		return 0, "file too small to identify"
	}

	switch {
	case bytes.Equal(header[:4], []byte{0x7f, 'E', 'L', 'F'}):
		return scoreELFBinary(header, goos, goarch)
	case header[0] == 'M' && header[1] == 'Z':
		return scorePEBinary(header, goos, goarch)
	default:
		if magic := machoMagic(header); magic != "" {
			return scoreMachOBinary(header, magic, goos, goarch)
		}
	}
	return 0, "unrecognized executable format; will try anyway"
}

func scoreELFBinary(header []byte, goos, goarch string) (int, string) {
	if len(header) < 20 {
		return 0, "truncated ELF header"
	}
	class := header[4]
	little := header[5] == 1
	machine := readUint16(header[18:20], little)
	expected := map[string]uint16{"amd64": 62, "386": 3, "arm": 40, "arm64": 183, "mips": 8}[goarch]
	archMatch := expected != 0 && machine == expected
	if class == 2 && (goarch == "386" || goarch == "arm") {
		archMatch = false
	}
	if class == 1 && (goarch == "amd64" || goarch == "arm64") {
		archMatch = false
	}
	osMatch := goos == "linux"
	switch {
	case osMatch && archMatch:
		return 3, "ELF matches " + goos + "/" + goarch
	case osMatch:
		return 2, "ELF but architecture differs"
	default:
		return 1, "ELF binary for another OS"
	}
}

func scorePEBinary(header []byte, goos, goarch string) (int, string) {
	if len(header) < 0x40 {
		return 0, "truncated PE header"
	}
	peOff := int(readUint32(header[0x3c:0x40], true))
	if peOff <= 0 || peOff+6 > len(header) || !bytes.Equal(header[peOff:peOff+4], []byte("PE\x00\x00")) {
		return 0, "invalid PE header"
	}
	machine := readUint16(header[peOff+4:peOff+6], true)
	expected := map[string]uint16{"amd64": 0x8664, "386": 0x14c, "arm64": 0xaa64}[goarch]
	archMatch := expected != 0 && machine == expected
	osMatch := goos == "windows"
	switch {
	case osMatch && archMatch:
		return 3, "PE matches " + goos + "/" + goarch
	case osMatch:
		return 2, "PE but architecture differs"
	default:
		return 1, "PE binary for another OS"
	}
}

func machoMagic(header []byte) string {
	if len(header) < 4 {
		return ""
	}
	if bytes.Equal(header[:4], []byte{0xca, 0xfe, 0xba, 0xbe}) {
		return "macho-fat"
	}
	switch readUint32(header[:4], true) {
	case 0xfeedface, 0xfeedfacf:
		return "macho-le"
	case 0xcefaedfe, 0xcffaedfe:
		return "macho-be"
	}
	return ""
}

func scoreMachOBinary(header []byte, magic, goos, goarch string) (int, string) {
	osMatch := goos == "darwin"
	archMatch := true
	if magic != "macho-fat" && len(header) >= 8 {
		var cpu uint32
		if magic == "macho-le" {
			cpu = readUint32(header[4:8], true)
		} else {
			cpu = readUint32(header[4:8], false)
		}
		expected := map[string]uint32{"amd64": 0x01000007, "arm64": 0x0100000c, "386": 7, "arm": 12}[goarch]
		archMatch = expected != 0 && cpu == expected
	}
	switch {
	case osMatch && archMatch:
		return 3, "Mach-O matches " + goos + "/" + goarch
	case osMatch:
		return 2, "Mach-O but architecture differs"
	default:
		return 1, "Mach-O binary for another OS"
	}
}

func readUint16(b []byte, little bool) uint16 {
	if len(b) < 2 {
		return 0
	}
	if little {
		return binary.LittleEndian.Uint16(b)
	}
	return binary.BigEndian.Uint16(b)
}

func readUint32(b []byte, little bool) uint32 {
	if len(b) < 4 {
		return 0
	}
	if little {
		return binary.LittleEndian.Uint32(b)
	}
	return binary.BigEndian.Uint32(b)
}
