package main

// BL-POC-006 指纹存储层单测：两源格式契约（模板切分/逐字节重生成）、条目增删改后
// 未动区域字节不变、手改吸收与坏 JSON 显式报错、表单序列化缩进契约。

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goby 形态 fixture：首行 `[{`、元素分隔 `, ` 同行、字段 2 空格、无尾换行（与真实库一致）。
const gobyFixture = `[{
  "name": "A",
  "logic": "a",
  "rule": [{
    "label": "a",
    "feature": "fa",
    "is_equal": true
  }]
}, {
  "name": "B",
  "logic": "a||b",
  "rule": [{
    "label": "a",
    "feature": "fb1",
    "is_equal": true
  }, {
    "label": "b",
    "feature": "fb2",
    "is_equal": false
  }]
}]`

// fingers_http 形态 fixture：元素 4 空格、字段 8 空格、分隔 `,\n    `、无尾换行。
const fingersFixture = `[
    {
        "focus": true,
        "link": "",
        "name": "A",
        "rule": [
            {
                "favicon": {
                    "mmh3": [
                        "123",
                        "-456"
                    ]
                }
            }
        ],
        "tag": [
            "cloud"
        ]
    },
    {
        "link": "http://x",
        "name": "B",
        "rule": [
            {
                "favicon": {
                    "md5": "abc"
                }
            }
        ]
    }
]`

func newFingerTestDB(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	db, err := openFingerDB(filepath.Join(t.TempDir(), "finger.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	dir := t.TempDir()
	gobyPath := filepath.Join(dir, "goby.json")
	fingersPath := filepath.Join(dir, "fingers_http.json")
	mustWrite(t, gobyPath, gobyFixture)
	mustWrite(t, fingersPath, fingersFixture)
	return db, gobyPath, fingersPath
}

// 模板切分+原样重生成 = 逐字节一致（两源）。
func TestFingerTemplateRoundTrip(t *testing.T) {
	db, gobyPath, fingersPath := newFingerTestDB(t)
	_ = db
	for _, path := range []string{gobyPath, fingersPath} {
		orig, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		tpl, elems, err := parseFingerArray(orig)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(elems) != 2 {
			t.Fatalf("%s: want 2 elems, got %d", path, len(elems))
		}
		if got := serializeFingerArray(tpl, elems); string(got) != string(orig) {
			t.Errorf("%s: round-trip not byte-identical", path)
		}
		if strings.Contains(tpl.Prefix, "name") {
			t.Errorf("%s: prefix captured element content: prefix=%q", path, tpl.Prefix)
		}
	}
}

// 删条目：明文中该条消失，其余区域逐字节不变（含分隔符原样）。
func TestFingerDelete_ByteContract(t *testing.T) {
	db, gobyPath, _ := newFingerTestDB(t)
	if err := seedFingerFile(db, "goby", []byte(gobyFixture)); err != nil {
		t.Fatal(err)
	}
	elems, _, err := fingerAllEntries(db, "goby")
	if err != nil {
		t.Fatal(err)
	}
	// 删第二条（B）：保留 A 原文，B 的间隙一并移除
	if err := fingerDelete(db, gobyPath, "goby", elems[1].ID); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(gobyPath)
	want := `[{` + strings.SplitN(gobyFixture[2:], `}, {`, 2)[0] + `}]`
	if string(got) != want {
		t.Errorf("after delete:\n got %q\nwant %q", got, want)
	}
}

// 编辑条目：未动条目字节不变、被编辑条目按所在源缩进契约重写。
func TestFingerEdit_ByteContract(t *testing.T) {
	db, gobyPath, fingersPath := newFingerTestDB(t)
	if err := seedFingerFile(db, "goby", []byte(gobyFixture)); err != nil {
		t.Fatal(err)
	}
	if err := seedFingerFile(db, "fingers_http", []byte(fingersFixture)); err != nil {
		t.Fatal(err)
	}
	elems, _, _ := fingerAllEntries(db, "goby")
	newB := strings.Replace(elems[1].Raw, `"fb2"`, `"fb2x"`, 1)
	if err := fingerEditRaw(db, gobyPath, "goby", elems[1].ID, "B", newB); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(gobyPath)
	if !strings.Contains(string(got), `"fb2x"`) || !strings.Contains(string(got), `"fa"`) {
		t.Errorf("edit lost content: %s", got)
	}
	if strings.Contains(string(got), `"fb2"`) {
		t.Errorf("stale content remained: %s", got)
	}
	// 逐字节契约：编辑后文件应恰为 fixture（仅 fb2→fb2x），未动区域（含元素间与规则数组内分隔符）原样
	if got2, _ := os.ReadFile(gobyPath); string(got2) != strings.Replace(gobyFixture, `"fb2"`, `"fb2x"`, 1) {
		t.Errorf("separator/untouched bytes disturbed: %s", got2)
	}

	// fingers 手改场景：文件加回一条 → 哈希失配吸收为 3 条 → 复活语义
	handEdited := strings.Replace(fingersFixture, `        "link": "http://x",`, `        "link": "http://x",
        "extra_field": 1,`, 1)
	if err := os.WriteFile(fingersPath, []byte(handEdited), 0644); err != nil {
		t.Fatal(err)
	}
	if err := absorbFingerFile(db, fingersPath, "fingers_http"); err != nil {
		t.Fatalf("absorb hand-edited: %v", err)
	}
	if n, _ := fingerCountDB(db, "fingers_http"); n != 2 {
		t.Errorf("absorb count = %d, want 2 (extra_field inside existing entry)", n)
	}
	regen, _ := os.ReadFile(fingersPath)
	if string(regen) != handEdited {
		t.Errorf("absorb+regenerate changed untouched hand-edited bytes")
	}
}

// 新增（追加超出原间隙数）后的重生成字节契约：末间隙衔接 + 未动区域原样——
// serializeFingerArray 间隙越界 panic 的回归锁（Reviewer CRITICAL 修复）。
func TestFingerAdd_ByteContract(t *testing.T) {
	db, gobyPath, _ := newFingerTestDB(t)
	if err := seedFingerFile(db, "goby", []byte(gobyFixture)); err != nil {
		t.Fatal(err)
	}
	newRaw := "{\n  \"name\": \"C\",\n  \"logic\": \"c\",\n  \"rule\": [{\n    \"label\": \"c\",\n    \"feature\": \"fc\",\n    \"is_equal\": true\n  }]\n}"
	if err := fingerAdd(db, gobyPath, "goby", "C", newRaw); err != nil {
		t.Fatalf("add+regenerate must not panic: %v", err)
	}
	got, _ := os.ReadFile(gobyPath)
	want := gobyFixture[:len(gobyFixture)-1] + ", " + newRaw + "]" // 末间隙 `, ` 衔接 + 原 `]` 后缀
	if string(got) != want {
		t.Errorf("add byte contract:\n got %q\nwant %q", got, want)
	}
}

// 手改清空为合法空数组：种子+重生成保留 '['（空数组前缀回归）。
func TestFingerEmptyArray_Template(t *testing.T) {
	db, _, fingersPath := newFingerTestDB(t)
	mustWrite(t, fingersPath, "[\n]")
	if err := absorbFingerFile(db, fingersPath, "fingers_http"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(fingersPath)
	if string(got) != "[\n]" {
		t.Errorf("empty array corrupted: %q", got)
	}
}

// 坏 JSON：显式报错且明文保持原状（不被清空/改写）。
func TestFingerAbsorb_BadJSON(t *testing.T) {
	db, _, fingersPath := newFingerTestDB(t)
	if err := seedFingerFile(db, "fingers_http", []byte(fingersFixture)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fingersPath, []byte("[broken"), 0644); err != nil {
		t.Fatal(err)
	}
	err := absorbFingerFile(db, fingersPath, "fingers_http")
	if err == nil || !strings.Contains(err.Error(), "解析失败") {
		t.Fatalf("want explicit parse error, got %v", err)
	}
	if got, _ := os.ReadFile(fingersPath); string(got) != "[broken" {
		t.Errorf("bad plaintext was mutated: %q", got)
	}
}

// 表单序列化：goby 字段 2 空格、fingers 字段 8 空格（prefix+indent）、首行 { 无缩进。
func TestSerializeFingerEntry_IndentContract(t *testing.T) {
	g, err := serializeFingerEntry("goby", []byte(`{"name":"X","logic":"a","rule":[{"label":"a","feature":"fx","is_equal":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(g, "{\n  \"name\": \"X\",\n") || !strings.HasSuffix(g, "\n}") || strings.HasPrefix(g, " ") {
		t.Errorf("goby indent contract broken:\n%s", g)
	}
	f, err := serializeFingerEntry("fingers_http", []byte(`{"name":"Y","focus":true,"rule":[{"favicon":{"md5":"abc"}}],"tag":["t1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(f, "{\n        \"focus\": true,\n") || !strings.HasSuffix(f, "\n    }") {
		t.Errorf("fingers indent contract broken:\n%s", f)
	}
	if _, err := serializeFingerEntry("goby", []byte(`{"name":"","logic":"a"}`)); err == nil {
		t.Errorf("empty name should be rejected")
	}
}
