package main

// BL-POC-008 内置 POC 写回编辑单测：findPocFile 三种 source 定位语义、内置写回的
// 三道守卫（存在性/改名/内容同一性）、groups 形态放行保存、原子写无残留。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	kibanaName = "poc-yaml-kibana-unauth"                // 内置库最小 GET 型 poc（embed 真源）
	kibanaFile = "kibana-unauth.yml"                     // embed 中该 poc 的真实文件名
	dubboName  = "poc-yaml-dubbo-admin-default-password" // groups 形态样本（无顶层 rules）
	dubboFile  = "dubbo-admin-default-password.yml"
)

// newPocTestServer 构造测试 server（root/pocsDir 各自独立临时目录，embed 用真源）。
func newPocTestServer(t *testing.T) *server {
	t.Helper()
	root := t.TempDir()
	pocDir := filepath.Join(root, "PocScan", "pocs")
	if err := os.MkdirAll(pocDir, 0755); err != nil {
		t.Fatal(err)
	}
	env, err := newCelEnv()
	if err != nil {
		t.Fatal(err)
	}
	return &server{repoRoot: root, pocsDir: t.TempDir(), celEnv: env}
}

// pocYAML 最小 rules 形态 poc yaml。
func pocYAML(name, path string) string {
	return "name: " + name + "\nrules:\n  - method: GET\n    path: " + path + "\n    expression: response.status == 200\n"
}

// putPocRaw 以 raw 模式请求调 putPoc（json.Marshal 构造请求体，yaml 换行安全转义）。
func putPocRaw(t *testing.T, s *server, fields map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/poc", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.putPoc(rec, req)
	return rec
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// 三种 source 定位语义：空=外置优先（旧行为）、external=只查外置、built-in=仓库源优先 embed 兜底。
func TestFindPocFile_SourceSemantics(t *testing.T) {
	s := newPocTestServer(t)
	mustWrite(t, filepath.Join(s.repoRoot, "PocScan", "pocs", kibanaFile), pocYAML(kibanaName, "/from-repo-source"))
	mustWrite(t, filepath.Join(s.pocsDir, kibanaFile), pocYAML(kibanaName, "/from-external"))

	cases := []struct {
		source   string
		wantPath string
		wantSrc  string
	}{
		{"", "/from-external", "external"},            // 旧行为：外置优先
		{"external", "/from-external", "external"},    // 只查外置
		{"built-in", "/from-repo-source", "built-in"}, // 仓库源优先（最新源）
	}
	for _, c := range cases {
		text, file, src, ok := s.findPocFile(kibanaName, c.source)
		if !ok || src != c.wantSrc || file != kibanaFile || !strings.Contains(text, c.wantPath) {
			t.Errorf("source=%q: got ok=%v src=%s file=%s textHit=%v, want src=%s path=%s", c.source, ok, src, file, strings.Contains(text, c.wantPath), c.wantSrc, c.wantPath)
		}
	}
}

// 内容同一性：-root 指向其他版本检出（同文件名装的是别的 POC）时 built-in 编辑视图回落 embed。
func TestFindPocFile_BuiltIn_IdentityFallback(t *testing.T) {
	s := newPocTestServer(t)
	mustWrite(t, filepath.Join(s.repoRoot, "PocScan", "pocs", kibanaFile), pocYAML("poc-yaml-some-other", "/other"))
	text, _, _, ok := s.findPocFile(kibanaName, "built-in")
	if !ok || !strings.Contains(text, "/app/kibana") {
		t.Errorf("identity mismatch should fall back to embed (contains /app/kibana), got ok=%v hit=%v", ok, strings.Contains(text, "/app/kibana"))
	}
}

// 写回内置源成功路径 + 原子写无残留。
func TestPutPoc_TargetBuiltIn_Happy(t *testing.T) {
	s := newPocTestServer(t)
	mustWrite(t, filepath.Join(s.repoRoot, "PocScan", "pocs", kibanaFile), pocYAML(kibanaName, "/old"))
	rec := putPocRaw(t, s, map[string]interface{}{
		"mode": "raw", "name": kibanaName, "target": "built-in", "file": kibanaFile,
		"yaml": pocYAML(kibanaName, "/new-marker"),
	})
	if !strings.Contains(rec.Body.String(), `"status":"saved"`) {
		t.Fatalf("save failed: %s", rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(s.repoRoot, "PocScan", "pocs", kibanaFile))
	if err != nil || !strings.Contains(string(got), "/new-marker") {
		t.Fatalf("source file not updated: err=%v content=%s", err, got)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(s.repoRoot, "PocScan", "pocs", "*.fscanx-web.tmp")); len(leftovers) != 0 {
		t.Errorf("atomic write leftover: %v", leftovers)
	}
}

// 写回内置源的全部守卫分支。
func TestPutPoc_TargetBuiltIn_Guards(t *testing.T) {
	t.Run("目标文件不存在", func(t *testing.T) {
		s := newPocTestServer(t) // root 不放 kibana 文件
		rec := putPocRaw(t, s, map[string]interface{}{"mode": "raw", "name": kibanaName, "target": "built-in", "yaml": pocYAML(kibanaName, "/x")})
		if !strings.Contains(rec.Body.String(), "内置源文件不存在") {
			t.Errorf("want missing-file rejection, got: %s", rec.Body.String())
		}
	})
	t.Run("改名守卫", func(t *testing.T) {
		s := newPocTestServer(t)
		mustWrite(t, filepath.Join(s.repoRoot, "PocScan", "pocs", kibanaFile), pocYAML(kibanaName, "/x"))
		rec := putPocRaw(t, s, map[string]interface{}{"mode": "raw", "name": kibanaName, "target": "built-in", "file": "someone-else.yml", "yaml": pocYAML(kibanaName, "/x")})
		if !strings.Contains(rec.Body.String(), "检测到重命名") {
			t.Errorf("want rename rejection, got: %s", rec.Body.String())
		}
	})
	t.Run("内容同一性守卫", func(t *testing.T) {
		s := newPocTestServer(t)
		mustWrite(t, filepath.Join(s.repoRoot, "PocScan", "pocs", kibanaFile), pocYAML("poc-yaml-some-other", "/x"))
		rec := putPocRaw(t, s, map[string]interface{}{"mode": "raw", "name": kibanaName, "target": "built-in", "file": kibanaFile, "yaml": pocYAML(kibanaName, "/x")})
		if !strings.Contains(rec.Body.String(), "覆盖错误文件") {
			t.Errorf("want identity rejection, got: %s", rec.Body.String())
		}
	})
	t.Run("内置库不存在的名", func(t *testing.T) {
		// 语义演进（用户裁决：web 新建资产可选"下次 go build 生效"）：embed 无此名不再拒绝——
		// 仓库源文件不存在 = 创建新文件（详细三场景见 TestPocPutBuiltInNewFile 等）
		s := newPocTestServer(t)
		rec := putPocRaw(t, s, map[string]interface{}{"mode": "raw", "name": "poc-yaml-brand-new", "target": "built-in", "yaml": pocYAML("poc-yaml-brand-new", "/x")})
		if strings.Contains(rec.Body.String(), `"code":1`) {
			t.Errorf("want create success, got: %s", rec.Body.String())
		}
		target := filepath.Join(s.repoRoot, "PocScan", "pocs", sanitizeName("poc-yaml-brand-new")+".yml")
		if _, err := os.Stat(target); err != nil {
			t.Errorf("内置源新文件未创建: %v", err)
		}
	})
}

// groups 形态（无顶层 rules，31/386）应可保存；rules/groups 皆空仍拒绝。
func TestPutPoc_GroupsForm(t *testing.T) {
	s := newPocTestServer(t)
	groupsYAML := "name: " + dubboName + "\ngroups:\n  root:\n    - method: GET\n      path: /\n      expression: response.status == 200\n"
	mustWrite(t, filepath.Join(s.repoRoot, "PocScan", "pocs", dubboFile), groupsYAML)
	rec := putPocRaw(t, s, map[string]interface{}{"mode": "raw", "name": dubboName, "target": "built-in", "file": dubboFile, "yaml": groupsYAML})
	if !strings.Contains(rec.Body.String(), `"status":"saved"`) {
		t.Fatalf("groups-form save should pass, got: %s", rec.Body.String())
	}

	rec2 := putPocRaw(t, s, map[string]interface{}{"mode": "raw", "name": kibanaName, "target": "external", "yaml": "name: " + kibanaName + "\n"})
	if !strings.Contains(rec2.Body.String(), "rules/groups 均为空") {
		t.Errorf("empty rules+groups should be rejected, got: %s", rec2.Body.String())
	}
}

// ---- 新建/转正写内置源（用户裁决：web 新建资产可选"下次 go build 生效"）----

// TestPocPutBuiltInNewFile embed 无此名 + 仓库源文件不存在 → 创建新文件（新建/转正写入）。
func TestPocPutBuiltInNewFile(t *testing.T) {
	s := newPocTestServer(t)
	name := "poc-yaml-brand-new-internal"
	rec := putPocRaw(t, s, map[string]interface{}{
		"mode": "raw", "name": name, "yaml": pocYAML(name, "/x"), "target": "built-in", "file": "",
	})
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("新建写入内置源被拒: %s", rec.Body.String())
	}
	target := filepath.Join(s.repoRoot, "PocScan", "pocs", sanitizeName(name)+".yml")
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("内置源新文件未创建: %v", err)
	}
	if !strings.Contains(string(raw), "name: "+name) {
		t.Fatalf("新文件内容不符: %s", string(raw))
	}
}

// TestPocPutBuiltInSameNameConsistent embed 无此名 + 仓库源文件同名且 name 一致 → 允许编辑
// （编辑未编译的仓库文件，与 getPoc built-in 读仓库最新源同语义）。
func TestPocPutBuiltInSameNameConsistent(t *testing.T) {
	s := newPocTestServer(t)
	name := "poc-yaml-uncompiled-repo-poc"
	target := filepath.Join(s.repoRoot, "PocScan", "pocs", sanitizeName(name)+".yml")
	mustWrite(t, target, pocYAML(name, "/old"))
	rec := putPocRaw(t, s, map[string]interface{}{
		"mode": "raw", "name": name, "yaml": pocYAML(name, "/new"), "target": "built-in", "file": "",
	})
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("编辑未编译仓库文件被拒: %s", rec.Body.String())
	}
	raw, _ := os.ReadFile(target)
	if !strings.Contains(string(raw), "/new") {
		t.Fatal("仓库文件内容未更新")
	}
}

// TestPocPutBuiltInSameNameConflict embed 无此名 + 仓库源文件同名但内容是其他 POC → 拒绝
// （防覆盖错文件），且原文件字节不动。
func TestPocPutBuiltInSameNameConflict(t *testing.T) {
	s := newPocTestServer(t)
	name := "poc-yaml-conflicting-file"
	other := "poc-yaml-some-other-poc"
	target := filepath.Join(s.repoRoot, "PocScan", "pocs", sanitizeName(name)+".yml")
	mustWrite(t, target, pocYAML(other, "/other"))
	before, _ := os.ReadFile(target)
	rec := putPocRaw(t, s, map[string]interface{}{
		"mode": "raw", "name": name, "yaml": pocYAML(name, "/x"), "target": "built-in", "file": "",
	})
	if !strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("同名异内容覆盖未被拒绝: %s", rec.Body.String())
	}
	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatal("拒绝写入后原文件字节被改动")
	}
}
