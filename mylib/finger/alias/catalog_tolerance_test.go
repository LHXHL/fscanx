package alias

import (
	"testing"

	"github.com/killmonday/fscanx/mylib/finger/resources"
	"gopkg.in/yaml.v3"
)

// 验证宽松解析对产品册条目新增 pocs 字段的容忍性（BL-POC-009 铁律第一验证项）：
// catalog.yaml 顶层保持现有列表结构（Unmarshal 目标 []*Alias 的前提），
// pocs 作为条目内未知字段由 yaml.v3 默认忽略——识别引擎解析零改动的前提。
// catalog.yaml 落地后本测试即为字节契约回归锁。
func TestUnmarshalToleratesCatalogPocsField(t *testing.T) {
	data := []byte(`- name: 泛微 e-cology
  vendor: weaver
  product: e-cology
  alias:
    goby:
      - 泛微-协同办公OA
  pocs:
    - poc-yaml-weaver-E-Cology-getSqlData-sqli
- name: tomcat
  vendor: apache
  product: tomcat
  alias:
    fingers:
      - tomcat
`)
	var aliases []*Alias
	if err := yaml.Unmarshal(data, &aliases); err != nil {
		t.Fatalf("含 pocs 字段的产品册条目解析失败: %v", err)
	}
	if len(aliases) != 2 {
		t.Fatalf("条目数不符: got %d want 2", len(aliases))
	}
	if aliases[0].Name != "泛微 e-cology" {
		t.Fatalf("标准名解析错: got %q", aliases[0].Name)
	}
	if aliases[1].Name != "tomcat" {
		t.Fatalf("无 pocs 条目解析错: got %q", aliases[1].Name)
	}
}

// TestCatalogSeeyonZhiyuanCoverage 致远系写法补录回归锁（BL-POC-009 用户裁决补录）：
// fingers/goby 两源的 7 个致远写法必须归一到「用友 Seeyon OA」——缺失则这些指纹路径
// 识别的目标按原名输出，seeyon 条目 11 个出厂测试静默不发（本 item 修复的失效模式）。
func TestCatalogSeeyonZhiyuanCoverage(t *testing.T) {
	as, err := NewAliases()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		engine string
		name   string
	}{
		{"fingers", "用友致远oa"},
		{"fingers", "致远OA-A6"},
		{"fingers", "致远OA-A8"},
		{"fingers", "致远oa A6"},
		{"fingers", "致远oa A8"},
		{"goby", "致远OA-A6"},
		{"goby", "致远OA-A8"},
	}
	for _, c := range cases {
		a, ok := as.Find(c.engine, resources.NormalizeString(c.name))
		if !ok {
			t.Errorf("写法 %s（%s 源）未归一——seeyon 出厂测试对该指纹路径静默失效", c.name, c.engine)
			continue
		}
		if a.Name != "用友 seeyon oa" {
			t.Errorf("写法 %s 归一目标不符: got %q", c.name, a.Name)
		}
	}
}
