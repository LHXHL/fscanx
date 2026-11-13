package Plugins

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/killmonday/fscanx/common"
)

func peTestHeader(machine uint16) []byte {
	b := make([]byte, 0x200)
	b[0], b[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(b[0x3c:0x40], 0x80)
	copy(b[0x80:0x84], []byte("PE\x00\x00"))
	binary.LittleEndian.PutUint16(b[0x84:0x86], machine)
	return b
}

func elfTestHeader(class byte, machine uint16) []byte {
	b := make([]byte, 64)
	copy(b[:4], []byte{0x7f, 'E', 'L', 'F'})
	b[4] = class
	b[5] = 1
	b[6] = 1
	binary.LittleEndian.PutUint16(b[18:20], machine)
	return b
}

func machoTestHeader(cpu uint32) []byte {
	b := make([]byte, 32)
	binary.LittleEndian.PutUint32(b[0:4], 0xfeedfacf)
	binary.LittleEndian.PutUint32(b[4:8], cpu)
	return b
}

func writeTestBinary(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestInspectMasscanBinary(t *testing.T) {
	dir := t.TempDir()
	pe := writeTestBinary(t, dir, "masscan.exe", peTestHeader(0x8664))
	elf := writeTestBinary(t, dir, "masscan_linux", elfTestHeader(2, 62))
	macho := writeTestBinary(t, dir, "masscan_mac", machoTestHeader(0x0100000c))
	bad := writeTestBinary(t, dir, "masscan_bad", []byte("not a binary"))

	cases := []struct {
		name      string
		path      string
		goos      string
		goarch    string
		wantScore int
	}{
		{"pe windows/amd64", pe, "windows", "amd64", 3},
		{"elf linux/amd64", elf, "linux", "amd64", 3},
		{"macho darwin/arm64", macho, "darwin", "arm64", 3},
		{"elf on windows", elf, "windows", "amd64", 1},
		{"unknown", bad, "windows", "amd64", 0},
	}
	for _, tc := range cases {
		score, reason := inspectMasscanBinary(tc.path, tc.goos, tc.goarch)
		if score != tc.wantScore {
			t.Errorf("%s: score = %d (%s), want %d", tc.name, score, reason, tc.wantScore)
		}
	}
}

func TestRankMasscanCandidates(t *testing.T) {
	dir := t.TempDir()
	writeTestBinary(t, dir, "masscan_linux", elfTestHeader(2, 62))
	writeTestBinary(t, dir, "masscan_bad", []byte("not a binary"))
	writeTestBinary(t, dir, "masscan.exe", peTestHeader(0x8664))

	candidates := collectMasscanCandidates([]string{dir}, "windows", "amd64")
	rankMasscanCandidates(candidates)
	if len(candidates) == 0 {
		t.Fatal("no candidates collected")
	}
	if got := filepath.Base(candidates[0].path); got != "masscan.exe" {
		t.Fatalf("first candidate = %s, want masscan.exe", got)
	}
	if candidates[0].score != 3 {
		t.Fatalf("masscan.exe score = %d, want 3", candidates[0].score)
	}
}

func hasString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestNormalizeMasscanTargets(t *testing.T) {
	var tokens []string
	for _, line := range []string{
		"10.0.0.0/8",
		"192.168.1.1-255",
		"192.168.1.1-192.168.1.5",
		"localhost",
		"http://localhost:8080/x",
		"HTTP://localhost:8080/x",
		"127.0.0.1:445",
		"127.0.0.3:445 # web",
		"Discovered open port 445/tcp on 127.0.0.2",
		"1.1.1.1,2.2.2.2",
	} {
		tokens = append(tokens, splitMasscanTargetLine(line)...)
	}

	targets, err := normalizeMasscanTargets(tokens)
	if err != nil {
		t.Fatalf("normalizeMasscanTargets: %v", err)
	}

	for _, want := range []string{
		"10.0.0.0/8",
		"192.168.1.1-192.168.1.255",
		"192.168.1.1-192.168.1.5",
		"127.0.0.1",
		"127.0.0.2",
		"127.0.0.3",
		"1.1.1.1",
		"2.2.2.2",
	} {
		if !hasString(targets, want) {
			t.Errorf("targets missing %q: %v", want, targets)
		}
	}
}

func TestNormalizeMasscanPorts(t *testing.T) {
	cases := []struct {
		raw     string
		exclude string
		want    string
		wantErr bool
	}{
		{"1-65535", "", "1-65535", false},
		{"80,443,1-3", "", "1-3,80,443", false},
		{"1-65535", "445", "1-444,446-65535", false},
		{"", "", "", true},
		{"80", "80", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeMasscanPorts(tc.raw, tc.exclude)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeMasscanPorts(%q,%q) = %q, want error", tc.raw, tc.exclude, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeMasscanPorts(%q,%q): %v", tc.raw, tc.exclude, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeMasscanPorts(%q,%q) = %q, want %q", tc.raw, tc.exclude, got, tc.want)
		}
	}

	web, err := normalizeMasscanPorts("web", "")
	if err != nil {
		t.Fatalf("normalizeMasscanPorts(web): %v", err)
	}
	if strings.Contains(web, "，") || !strings.Contains(web, "80") || !strings.Contains(web, "443") {
		t.Fatalf("web ports not normalized as expected: %q", web)
	}
}

func TestParseMasscanStatus(t *testing.T) {
	common.StopProgress()
	input := "rate: 0.00-kpps, 50.00% done, 829:09:53 remaining, found=0      \r" +
		"rate: 1.00-kpps, 100.00% done, waiting 1-secs, found=7 \r" +
		"ERROR: bad adapter\r"

	status := &masscanStatus{}
	parseMasscanStderr(strings.NewReader(input), status)

	if status.Found != 7 {
		t.Fatalf("found = %d, want 7", status.Found)
	}
	if !strings.Contains(status.Last, "waiting 1-secs") {
		t.Fatalf("last status = %q, want waiting 1-secs", status.Last)
	}
	if !strings.Contains(status.Tail(), "bad adapter") {
		t.Fatalf("tail = %q, want bad adapter", status.Tail())
	}
}

func TestParseMasscanExtraArgs(t *testing.T) {
	if args, err := parseMasscanExtraArgs("--wait 3 --adapter-ip 10.0.0.1"); err != nil || len(args) != 4 {
		t.Fatalf("valid args = %v, %v", args, err)
	}
	for _, raw := range []string{"-p80", "--rate=1000", "-iL hosts.txt"} {
		if _, err := parseMasscanExtraArgs(raw); err == nil {
			t.Errorf("parseMasscanExtraArgs(%q) = nil, want error", raw)
		}
	}
}

func TestWriteMasscanTargetFileReadOnly(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "hosts.txt")
	content := []byte("localhost,1.1.1.1\n# comment\n127.0.0.1:445\n")
	if err := os.WriteFile(input, content, 0644); err != nil {
		t.Fatal(err)
	}

	targetFile, count, err := writeMasscanTargetFile("", input)
	if err != nil {
		t.Fatalf("writeMasscanTargetFile: %v", err)
	}
	defer os.Remove(targetFile)

	after, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, after) {
		t.Fatalf("input file was modified: before=%q after=%q", content, after)
	}
	if count < 2 {
		t.Fatalf("target count = %d, want >= 2", count)
	}

	targets, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(targets, []byte("127.0.0.1")) || !bytes.Contains(targets, []byte("1.1.1.1")) {
		t.Fatalf("target file content = %q", targets)
	}
}

func TestFindMasscanBinaryUserPath(t *testing.T) {
	dir := t.TempDir()
	path := writeTestBinary(t, dir, "masscan_fake", peTestHeader(0x8664))
	got, err := findMasscanBinary(path, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("findMasscanBinary: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("path = %q, want absolute", got)
	}
	if _, err := findMasscanBinary(filepath.Join(dir, "missing"), runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("missing -mp path should error")
	}
}

func TestWriteMasscanTargetFileHostList(t *testing.T) {
	path, count, err := writeMasscanTargetFile("10.0.0.0/8,127.0.0.1", "")
	if err != nil {
		t.Fatalf("writeMasscanTargetFile: %v", err)
	}
	defer os.Remove(path)

	if count != 2 {
		t.Fatalf("target count = %d, want 2", count)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("10.0.0.0/8")) || !bytes.Contains(content, []byte("127.0.0.1")) {
		t.Fatalf("target file content = %q", content)
	}
}

func TestWriteMasscanExcludeFile(t *testing.T) {
	path, err := writeMasscanExcludeFile("10.0.0.0/8,127.0.0.1")
	if err != nil {
		t.Fatalf("writeMasscanExcludeFile: %v", err)
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("10.0.0.0/8")) || !bytes.Contains(content, []byte("127.0.0.1")) {
		t.Fatalf("exclude file content = %q", content)
	}

	if empty, err := writeMasscanExcludeFile(""); err != nil || empty != "" {
		t.Fatalf("empty -hn = %q, %v; want empty, nil", empty, err)
	}
}

func TestNormalizeMasscanTargetTokenRejects(t *testing.T) {
	for _, value := range []string{"fe80::1", "999.1.1.1", "10.0.0.0/99", "192.168.1.5-192.168.1.1"} {
		if _, err := normalizeMasscanTargetToken(value); err == nil {
			t.Errorf("normalizeMasscanTargetToken(%q) = nil, want error", value)
		}
	}
}

func TestSplitMasscanTargetLine(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"127.0.0.1:445 # web", []string{"127.0.0.1:445"}},
		{"10.0.0.0/8 # corp", []string{"10.0.0.0/8"}},
		{"1.2.3.4 445", []string{"1.2.3.4"}},
		{"1.1.1.1,2.2.2.2", []string{"1.1.1.1", "2.2.2.2"}},
		{"# comment", nil},
	}
	for _, tc := range cases {
		got := splitMasscanTargetLine(tc.line)
		if len(got) != len(tc.want) {
			t.Errorf("splitMasscanTargetLine(%q) = %v, want %v", tc.line, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitMasscanTargetLine(%q) = %v, want %v", tc.line, got, tc.want)
				break
			}
		}
	}
}
