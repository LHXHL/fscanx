package PocScan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/killmonday/fscanx/PocScan/lib"
)

func writeTempBinds(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "binds.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadBinds 验收条件：binds.yaml 解析与 always 标记（BL-POC-002）。
func TestLoadBinds(t *testing.T) {
	t.Run("文件不存在返回nil正常态", func(t *testing.T) {
		binds, err := LoadBinds(filepath.Join(t.TempDir(), "not-exist.yaml"))
		if err != nil || binds != nil {
			t.Fatalf("want nil,nil got %v,%v", binds, err)
		}
	})

	t.Run("正常解析与未知字段忽略", func(t *testing.T) {
		// always 已迁至 poc yaml 自身（BL-POC-004 迭代轮）；旧 binds.yaml 残留 always 键
		// 按 yaml 非严格解析忽略，不报错不炸文件。
		path := writeTempBinds(t, `binds:
  - product: 若依-ruoyi
    pocs:
      - poc-yaml-ruoyi-backdoor
  - product: 通达OA
    pocs:
      - poc-yaml-tongda-xxx
  - product: 通用目录探测
    always: true
    pocs:
      - poc-yaml-git-leak
`)
		binds, err := LoadBinds(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(binds) != 3 {
			t.Fatalf("want 3 binds got %d", len(binds))
		}
		if binds[0].Product != "若依-ruoyi" || len(binds[0].Pocs) != 1 || binds[0].Pocs[0] != "poc-yaml-ruoyi-backdoor" {
			t.Fatalf("binds[0] mismatch: %+v", binds[0])
		}
	})

	t.Run("空文件返回nil", func(t *testing.T) {
		binds, err := LoadBinds(writeTempBinds(t, ""))
		if err != nil || binds != nil {
			t.Fatalf("want nil,nil got %v,%v", binds, err)
		}
	})

	t.Run("非法yaml报错", func(t *testing.T) {
		binds, err := LoadBinds(writeTempBinds(t, "binds: [unclosed"))
		if err == nil || binds != nil {
			t.Fatalf("want error got %v,%v", binds, err)
		}
	})
}

// TestBindsSeed 内置兜底可解析且非空（BL-POC-003：出厂兜底保证智能定向不因缺外置文件失效；
// BL-POC-009 起种子源 = 出厂产品册 catalog.yaml 的 pocs 段投影）。
func TestBindsSeed(t *testing.T) {
	binds, err := catalogSeed()
	if err != nil {
		t.Fatal(err)
	}
	if len(binds) == 0 {
		t.Fatal("catalog binds is empty")
	}
	for _, b := range binds {
		if b.Product == "" || len(b.Pocs) == 0 {
			t.Fatalf("bad seed entry: %+v", b)
		}
	}
}

// TestMergeBinds 外置 binds.yaml 两层合并：同 product 整条覆盖、异名追加（BL-POC-003）。
func TestMergeBinds(t *testing.T) {
	seed := []Bind{
		{Product: "thinkphp", Pocs: []string{"poc-a"}},
		{Product: "seeyon", Pocs: []string{"poc-b"}},
	}
	outer := []Bind{
		{Product: "thinkphp", Pocs: []string{"poc-a2"}},                         // 覆盖
		{Product: "tongda", Pocs: []string{"poc-c"}},              // 追加
	}
	merged := MergeBinds(seed, outer)
	if len(merged) != 3 {
		t.Fatalf("want 3 got %d", len(merged))
	}
	if merged[0].Product != "thinkphp" || len(merged[0].Pocs) != 1 || merged[0].Pocs[0] != "poc-a2" {
		t.Fatalf("override mismatch: %+v", merged[0])
	}
	if merged[1].Product != "seeyon" || merged[1].Pocs[0] != "poc-b" {
		t.Fatalf("seed untouched mismatch: %+v", merged[1])
	}
	if merged[2].Product != "tongda" {
		t.Fatalf("append mismatch: %+v", merged[2])
	}
}

// TestBoundPocNames 定向查询：命中产品条目 pocs 并集；无命中为零（BL-POC-003）。
func TestBoundPocNames(t *testing.T) {
	orig := PocBinds
	defer func() { PocBinds = orig }()

	PocBinds = []Bind{
		{Product: "thinkphp", Pocs: []string{"poc-tp1", "poc-tp2"}},
		{Product: "seeyon", Pocs: []string{"poc-seeyon"}},
	}
	got := boundPocNames([]string{"[*thinkphp]", "php"})
	if len(got) != 2 {
		t.Fatalf("want 2 names got %v", got)
	}
	if got[0] != "poc-tp1" || got[1] != "poc-tp2" {
		t.Fatalf("names mismatch: %v", got)
	}
	if n := boundPocNames([]string{"nginx"}); len(n) != 0 {
		t.Fatalf("no-hit should be empty: %v", n)
	}
	if n := boundPocNames(nil); len(n) != 0 {
		t.Fatalf("nil frames should be empty: %v", n)
	}
}

// TestAlwaysPocNames 无条件全打型：poc yaml 自身 always 标记（从 binds 迁移，BL-POC-004 迭代轮）。
func TestAlwaysPocNames(t *testing.T) {
	orig := AllPocs
	defer func() { AllPocs = orig }()

	AllPocs = []*lib.Poc{
		{Name: "poc-normal"},
		{Name: "poc-git", Always: true},
		{Name: "poc-dir", Always: true},
	}
	got := alwaysPocNames()
	if len(got) != 2 || got[0] != "poc-git" || got[1] != "poc-dir" {
		t.Fatalf("always names mismatch: %v", got)
	}
	AllPocs = nil
	if n := alwaysPocNames(); len(n) != 0 {
		t.Fatalf("empty AllPocs should be empty: %v", n)
	}
}
