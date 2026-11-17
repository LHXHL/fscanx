//go:build ignore

// gen_catalog 生成出厂产品册草稿与归一核对表（BL-POC-009 Phase 1）。
//
// **迁移使命已完成（2026-11-17）**：草稿经用户核对固化为 mylib/finger/resources/catalog.yaml，
// 旧两文件（aliases.yaml / binds-seed.yaml）已删除——本脚本读取旧 seed 的数据源已不存在，
// 不可重跑（重跑需改读 catalog.yaml 自身，出厂产品册转人工维护；如需重生成另立工具）。
// 保留本文件作为拆分决策（splitRules/extraAliases/staleAliases 含依据注释）与归一核对
// 口径（ToLower 输出形态、拼音归一）的历史档案。
//
// 职责：
//  1. 合并出厂两文件（aliases.yaml 别名表 + binds-seed.yaml 预置名单）为产品册草稿
//     catalog.yaml.draft——条目 = 标准名 + 别名 + 测试名单（pocs）；
//  2. weaver 预置名单拆分子产品（拆分依据见 splitRules 注释，路径特征实证）；
//  3. 补漏收别名（引擎归一核对发现的指纹库真实写法未收录项）；
//  4. 输出归一核对表（两明文指纹库每个 name 经引擎 Map 的归一去向）供人工核对。
//
// 运行：go run plans/scripts/gen_catalog.go
// 产物：mylib/finger/resources/catalog.yaml.draft + plans/catalog-核对表.md
// 维护口径：草稿经用户人工核对后固化为 catalog.yaml（重命名去 .draft），本脚本退役。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	fingers "github.com/killmonday/fscanx/mylib/finger"
	"github.com/killmonday/fscanx/mylib/finger/alias"
	"github.com/killmonday/fscanx/mylib/finger/resources"
	"gopkg.in/yaml.v3"
)

// draftCatalogEntry 产品册草稿条目（引擎 Alias 结构 + pocs 扩展字段；
// 引擎宽松解析忽略 pocs——mylib/finger/alias/catalog_tolerance_test.go 回归锁）。
type draftCatalogEntry struct {
	Name    string              `yaml:"name"`
	Vendor  string              `yaml:"vendor,omitempty"`
	Product string              `yaml:"product,omitempty"`
	Alias   map[string][]string `yaml:"alias,omitempty"`
	Pocs    []string            `yaml:"pocs,omitempty"`
}

type seedFile struct {
	Bind []struct {
		Product string   `yaml:"product"`
		Pocs    []string `yaml:"pocs"`
	} `yaml:"binds"`
}

// splitRules weaver 预置名单拆分决策（人工核对产物，依据 = poc 规则路径特征实证）：
//   - getSqlData-sqli:          E-Cology getSqlData.jsp 数据泄露
//   - arbitrary-file-upload:    /page/exportImport/uploadOperation.jsp —— e-cology 页面特征
//   - ebridge-file-read:        云桥 e-Bridge weaver.ebridge 组件
//   - eoffice-v9-upload-getshell: /general/index/UploadFile.php?...eoffice_logo —— e-office 产品特征
var splitRules = map[string]string{ // poc 名子串 → 产品册标准名
	"getSqlData-sqli":    "泛微 e-cology",
	"arbitrary-file-":    "泛微 e-cology",
	"ebridge-file-read":  "泛微 云桥e-bridge",
	"eoffice-v9-upload-": "泛微 e-office",
}

// selfStd 自标准名条目：原 seed 英文键无中文标准名对应（指纹库无别名条目，引擎输出原始
// 英文名），按原名立条挂 pocs—— Contains(输出原名, 绑定原名) 相等命中，行为与现状一致。
var selfStd = map[string]string{ // seed 键 → 产品册标准名
	"thinkphp": "thinkphp", "nexus": "nexus", "qizhi": "qizhi",
	"weblogic": "weblogic", "zabbix": "zabbix", "vmware": "vmware",
	"jboss": "jboss", "coremail": "coremail", "kingsoft": "kingsoft",
}

// seedToStd 旧 seed 键 → 现有别名表标准名（已生效或按标准名重挂）。
var seedToStd = map[string]string{
	"seeyon":  "用友 Seeyon OA", // 标准名含 seeyon 子串，现状已生效，语义保持
	"tongda":  "通达 OA",
	"landray": "蓝凌 OA",
}

// extraAliases 漏收补充（引擎归一核对发现的指纹库真实写法，2026-11-17 草稿轮）：
// 泛微 云桥e-bridge ← goby "泛微-云桥e-Bridge"（现有条目只收 ehole/fingerprinthub 写法）；
// 泛微 e-mobile ← goby "泛微-EMobile"（现有 goby 别名"泛微-移动办公OA"指向不存在的写法，替换）。
var extraAliases = map[string]map[string][]string{
	"泛微 云桥e-bridge": {"goby": {"泛微-云桥e-Bridge"}},
	"泛微 e-mobile":    {"goby": {"泛微-EMobile"}},
	"泛微 e-office":    {"goby": {"泛微-EOffice"}},
	// 引擎归一核对表（plans/catalog-核对表.md）人工裁决收录——2026-11-17 草稿轮
	"通达 OA":   {"fingers": {"tongda"}, "goby": {"TDXK-通达OA"}, "ehole": {"Tongda", "通达OA"}},
	"coremail":  {"goby": {"Coremail-邮件系统"}},
	"jboss":     {"goby": {"JBoss-5.0", "JBoss-AS", "JBoss-EAP"}, "fingers": {"jboss application server 7"}},
	"weblogic":  {"goby": {"BEA-WebLogic-Server", "Weblogic_interface_7001"}},
	"nexus":     {"goby": {"Nexus-Repository-Manager"}, "fingers": {"sonatype nexus repository manager"}},
	"thinkphp":  {"goby": {"ThinkPHP", "ThinkPHP-YFCMF"}},
	"zabbix":    {"goby": {"ZABBIX-监控系统"}},
	"qizhi":     {"fingers": {"齐治堡垒机"}, "goby": {"Qi Zhi Technology - Fortress"}, "ehole": {"齐治堡垒机"}},
	"蓝凌 OA":    {"fingers": {"蓝凌OA-EKP"}, "goby": {"蓝凌OA-EKP"}},
	"协众OA":     {"fingers": {"协众软件-协众OA"}, "goby": {"协众软件-协众OA"}},
	// 用友U8 写法：用户 2026-11-17 裁决收录（goby 特征 /indexbusiness.jsp 为 yyoa 业务页，
	// 判定为 U8 OA 而非 U8 ERP；接受其中潜在 ERP 站点的少量误打换取绑定修复完整性）
	"用友 U8 OA": {"fingers": {"用友U8"}, "goby": {"用友U8", "用友-U8"}},
	// gz 源复审补录（2026-11-17 对抗式复审：fingerprinthub/ehole 解压逐写法归一键核对，
	// 只收 Contains 不命中且条目有 pocs 的——修复绑定命中；英文变体 Weblogic/Jboss 等
	// Contains 自标准名已命中不收，保持克制）
	"泛微 e-cology": {"fingerprinthub": {"ecology-oa"}, "ehole": {"ecology", "泛微 OA"}}, // ecology-oa 特征 /wui/（e-cology 前端路径）实锤；"泛微 OA" 归 e-cology（泛微主体产品，轻微歧义已知）
	"用友 Seeyon OA": {"ehole": {"致远OA"}}, // 致远互联=Seeyon 同产品异名；M1/M3/Analytics 子产品不收
}

// staleAliases 失效别名清理（指向指纹库中不存在的写法；原表错收，保留会污染归一表与界面展示）。
var staleAliases = map[string][]string{
	"泛微 e-mobile": {"泛微-移动办公OA"}, // goby 源无此写法（真实写法是"泛微-EMobile"），原表错收
}

var nameRe = regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)

func main() {
	os.Exit(run())
}

func run() int {
	// ── 1. 读基底：迁移期旧 aliases.yaml 已删，现以固化 catalog.yaml 为基底增量补录
	// （extraAliases/staleAliases 幂等应用——appendAlias 去重、removeNames 顺序保持）。
	// 迁移首跑场景（旧 aliases.yaml 仍在）自动优先旧表。
	base := "mylib/finger/resources/catalog.yaml"
	if _, err := os.Stat("mylib/finger/resources/aliases.yaml"); err == nil {
		base = "mylib/finger/resources/aliases.yaml" // 迁移首跑：旧表为基底
	}
	rawAlias, err := os.ReadFile(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read base catalog:", err)
		return 1
	}
	var entries []draftCatalogEntry
	if err := yaml.Unmarshal(rawAlias, &entries); err != nil {
		fmt.Fprintln(os.Stderr, "parse base catalog:", err)
		return 1
	}

	// ── 2. 读旧 seed 名单（迁移首跑用；固化后 binds-seed.yaml 已退役删除，seedFile 空
	// = 拆分段跳过——pocs 已在基底条目中）──
	var seed seedFile
	if rawSeed, serr := os.ReadFile("PocScan/binds-seed.yaml"); serr == nil {
		if perr := yaml.Unmarshal(rawSeed, &seed); perr != nil {
			fmt.Fprintln(os.Stderr, "parse binds-seed.yaml:", perr)
			return 1
		}
	}

	// ── 3. 内置 poc 库 name 全集（pocs 拆分与自标准名挂载用）──
	pocNames := pocNameSet("PocScan/pocs")

	// ── 4. 合并出产品册草稿 ──
	byName := map[string]*draftCatalogEntry{}
	var order []string
	for i := range entries {
		e := entries[i]
		cp := e
		byName[cp.Name] = &cp
		order = append(order, cp.Name)
	}
	// 4a. weaver 拆分（旧 weaver/weaver-ebridge/yongyou 键退役，pocs 按拆分表重挂）
	pocsToStd := map[string]string{}
	for _, b := range seed.Bind {
		switch b.Product {
		case "weaver", "weaver-ebridge", "yongyou":
			for _, p := range b.Pocs {
				std := ""
				for frag, target := range splitRules {
					if strings.Contains(p, frag) {
						std = target
						break
					}
				}
				if std == "" && b.Product == "yongyou" {
					std = "用友 U8 OA" // u8-oa-sqli：路径 /yyoa/ + PeiQi wiki 用友 U8 OA 实证
				}
				if std == "" {
					fmt.Printf("[!] poc %s (seed=%s) 无拆分规则，滞留人工核对\n", p, b.Product)
					continue
				}
				pocsToStd[p] = std
			}
		case "seeyon":
			for _, p := range b.Pocs {
				pocsToStd[p] = seedToStd["seeyon"]
			}
		case "tongda":
			for _, p := range b.Pocs {
				pocsToStd[p] = seedToStd["tongda"]
			}
		case "landray":
			for _, p := range b.Pocs {
				pocsToStd[p] = seedToStd["landray"]
			}
		default:
			std := selfStd[b.Product]
			if std == "" {
				fmt.Printf("[!] seed 键 %s 无映射，滞留人工核对\n", b.Product)
				continue
			}
			for _, p := range b.Pocs {
				pocsToStd[p] = std
			}
		}
	}
	// 4b. pocs 挂载到条目（新条目按需创建）
	ensure := func(std string) *draftCatalogEntry {
		if e, ok := byName[std]; ok {
			return e
		}
		e := &draftCatalogEntry{Name: std}
		byName[std] = e
		order = append(order, std)
		return e
	}
	for p, std := range pocsToStd {
		if !pocNames[p] {
			fmt.Printf("[!] 挂载 poc %s 不在内置库 name 全集（名字拼写核对）\n", p)
		}
		e := ensure(std)
		e.Pocs = append(e.Pocs, p)
	}
	// 4c. 漏收别名补充
	for std, m := range extraAliases {
		e := ensure(std)
		for eng, names := range m {
			e.Alias = appendAlias(e.Alias, eng, names)
		}
	}
	// 4d. 失效别名清理（工具内维护——草稿是生成物，手改会被重跑覆盖）
	for std, names := range staleAliases {
		if e := byName[std]; e != nil {
			for eng := range e.Alias {
				e.Alias[eng] = removeNames(e.Alias[eng], names)
			}
		}
	}
	// 4d. e-office / U8 OA 新条目的 vendor/product 元数据（与现有条目惯例对齐）
	if e := byName["泛微 e-office"]; e != nil {
		e.Vendor, e.Product = "weaver", "e-office"
	}
	if e := byName["用友 U8 OA"]; e != nil {
		e.Vendor, e.Product = "yonyou", "U8 OA"
	}

	// ── 5. 渲染产品册草稿 ──
	sort.Strings(order)
	var b strings.Builder
	b.WriteString("# 出厂产品册（BL-POC-009：aliases.yaml + binds-seed.yaml 合并重刷）。\n")
	b.WriteString("# 条目 = 标准名 + 别名（各指纹源写法归一到标准名）+ 测试名单（pocs，绑定兜底）。\n")
	b.WriteString("# 顶层保持列表结构（引擎解析 []*Alias 的前提）；pocs 为引擎忽略的扩展字段。\n")
	b.WriteString("# 由 plans/scripts/gen_catalog.go 生成草稿，经人工核对后固化。\n")
	for _, name := range order {
		e := byName[name]
		b.WriteString("- name: " + yamlScalar(name) + "\n")
		if e.Vendor != "" {
			b.WriteString("  vendor: " + yamlScalar(e.Vendor) + "\n")
		}
		if e.Product != "" {
			b.WriteString("  product: " + yamlScalar(e.Product) + "\n")
		}
		if len(e.Alias) > 0 {
			b.WriteString("  alias:\n")
			engines := make([]string, 0, len(e.Alias))
			for eng := range e.Alias {
				engines = append(engines, eng)
			}
			sort.Strings(engines)
			for _, eng := range engines {
				b.WriteString("    " + eng + ":\n")
				for _, n := range e.Alias[eng] {
					b.WriteString("      - " + yamlScalar(n) + "\n")
				}
			}
		}
		if len(e.Pocs) > 0 {
			ps := append([]string(nil), e.Pocs...)
			sort.Strings(ps)
			b.WriteString("  pocs:\n")
			for _, p := range ps {
				b.WriteString("    - " + p + "\n")
			}
		}
	}
	if err := os.WriteFile("mylib/finger/resources/catalog.yaml.draft", []byte(b.String()), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write draft:", err)
		return 1
	}
	fmt.Printf("[+] 产品册草稿 %d 条目 → mylib/finger/resources/catalog.yaml.draft\n", len(order))

	// ── 6. 归一核对表（引擎真实归一逻辑）──
	return auditNormalization(byName)
}

// auditNormalization 两明文指纹库全部 name 经引擎 Aliases.Map 求归一去向，输出核对表：
// 到达产品册标准名 / 自标准名（输出=原名）/ 未归属。疑似漏收 = 写法名与产品册标准名
// 规范化后互为包含（中文拼音归一口径），列候选供人工裁决。
func auditNormalization(byName map[string]*draftCatalogEntry) int {
	eng, err := fingers.NewEngine("fingers", "goby")
	if err != nil {
		fmt.Fprintln(os.Stderr, "new engine:", err)
		return 1
	}
	// 注入草稿别名：核对表模拟"草稿固化为 catalog.yaml 后"的归一行为
	// （引擎 embed 真源仍是旧 aliases.yaml，不注入则已裁决收录项被误报为漏收）。
	if draftData, derr := os.ReadFile("mylib/finger/resources/catalog.yaml.draft"); derr == nil {
		var draft []draftCatalogEntry
		if perr := yaml.Unmarshal(draftData, &draft); perr == nil {
			inject := make([]*alias.Alias, 0, len(draft))
			for _, e := range draft {
				inject = append(inject, &alias.Alias{
					Name: e.Name, Vendor: e.Vendor, Product: e.Product, AliasMap: e.Alias,
				})
			}
			eng.Aliases.AppendAliases(inject)
		}
	}
	// 产品册标准名的规范化集合（引擎实际输出 = Compile 后 ToLower 形态的条目名，
	// alias.go:42——到达判定必须同口径 ToLower，否则大量假"未到达"）
	stdLower := map[string]string{} // ToLower(标准名) → 标准名
	stdNorm := map[string]string{}  // NormalizeString(标准名) → 标准名（疑似判定用）
	for name := range byName {
		stdLower[strings.ToLower(name)] = name
		stdNorm[resources.NormalizeString(name)] = name
	}

	type row struct {
		source, name, out string
		reaches           bool // 归一输出 ∈ 产品册标准名
		suspect           bool // 疑似产品册产品写法（规范化互含）但未到达
	}
	var rows []row
	sources := map[string]string{
		"goby.json/goby.json":           "goby",
		"fingers_http.json/fingers_http.json": "fingers",
	}
	for rel, engineKey := range sources {
		data, err := os.ReadFile(filepath.Join("mylib/finger/resources", rel))
		if err != nil {
			fmt.Fprintln(os.Stderr, "read", rel, err)
			return 1
		}
		// 两明文都是 JSON 数组，只提取 name 字段（结构无关，长尾字段不碍事）
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(data, &arr); err != nil {
			fmt.Fprintln(os.Stderr, "parse", rel, err)
			return 1
		}
		for _, item := range arr {
			var name string
			_ = json.Unmarshal(item["name"], &name)
			if name == "" {
				continue
			}
			out := name
			if aliasName, ok := eng.Aliases.Map[engineKey][resources.NormalizeString(name)]; ok {
				out = aliasName
			}
			r := row{source: engineKey, name: name, out: out}
			if _, ok := stdLower[strings.ToLower(out)]; ok {
				r.reaches = true
			} else {
				nOut := resources.NormalizeString(out)
				for nStd, std := range stdNorm {
					if strings.Contains(nOut, nStd) || strings.Contains(nStd, nOut) {
						r.suspect = true
						_ = std
						break
					}
				}
			}
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].source != rows[j].source {
			return rows[i].source < rows[j].source
		}
		if rows[i].reaches != rows[j].reaches {
			return !rows[i].reaches // 未到达在前
		}
		if rows[i].suspect != rows[j].suspect {
			return rows[i].suspect // 疑似漏收次前
		}
		return rows[i].name < rows[j].name
	})

	var b strings.Builder
	b.WriteString("# 归一核对表（BL-POC-009 Phase 1，引擎真实归一逻辑实测）\n\n")
	b.WriteString("裁决原则（2026-11-17）：只收录**有 pocs 绑定条目**的漏收写法（绑定命中修复导向）；无绑定条目的写法变体（NC/GRP-U8/TurboCRM/金蝶云星空/红帆等子产品系）与同名异产品（vmware 应用系 vs ESXi/NSX、思科 Nexus vs Sonatype Nexus）**不收**——误归一会扩大绑定误打面。已收项见 gen_catalog.go extraAliases（含注释依据），失效清理见 staleAliases。\n\n")
	b.WriteString("下表为收录后基线的剩余漏收段（均为不收裁决：同名异产品/无绑定条目的子产品变体，见裁决原则）。\n\n")
	b.WriteString("## 未到达且疑似漏收（需人工裁决）\n\n")
	b.WriteString("| 源 | 指纹库写法 | 引擎输出 | 疑似标准名 |\n|---|---|---|---|\n")
	b.WriteString("## 未到达且疑似漏收（需人工裁决）\n\n")
	b.WriteString("| 源 | 指纹库写法 | 引擎输出 | 疑似标准名 |\n|---|---|---|---|\n")
	for _, r := range rows {
		if r.reaches || !r.suspect {
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | — |\n", r.source, r.name, r.out))
	}
	b.WriteString("\n## 到达产品册标准名（核对归类正确）\n\n")
	b.WriteString("| 源 | 指纹库写法 | 引擎输出 |\n|---|---|---|\n")
	for _, r := range rows {
		if !r.reaches {
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.source, r.name, r.out))
	}
	b.WriteString("\n## 自标准名（输出原名，统计）\n\n")
	count := 0
	for _, r := range rows {
		if r.reaches || r.suspect {
			continue
		}
		count++
	}
	b.WriteString(fmt.Sprintf("共 %d 条，输出原名参与 Contains 匹配，行为不变，不再逐条列出。\n", count))
	if err := os.WriteFile("plans/catalog-核对表.md", []byte(b.String()), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write audit:", err)
		return 1
	}
	fmt.Printf("[+] 归一核对表 → plans/catalog-核对表.md（%d 行指纹名实测）\n", len(rows))
	return 0
}

// pocNameSet 目录下全部 poc yaml 的 name 字段（gen_binds_seed 同款正则，零依赖）。
func pocNameSet(dir string) map[string]bool {
	set := map[string]bool{}
	re := regexp.MustCompile(`(?m)^name:\s*(\S+)`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return set
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if m := re.FindSubmatch(raw); m != nil {
			set[string(m[1])] = true
		}
	}
	return set
}

func appendAlias(m map[string][]string, engine string, names []string) map[string][]string {
	if m == nil {
		m = map[string][]string{}
	}
set:
	for _, n := range names {
		for _, have := range m[engine] {
			if have == n {
				continue set
			}
		}
		m[engine] = append(m[engine], n)
	}
	return m
}

// removeNames 从别名清单剔除失效写法（顺序保持）。
func removeNames(list, names []string) []string {
	out := list[:0]
	for _, item := range list {
		drop := false
		for _, n := range names {
			if item == n {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, item)
		}
	}
	return out
}

// yamlScalar 含冒号/井号等特殊字符的标量加引号，其余原样（可读性优先）。
func yamlScalar(s string) string {
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`") || strings.HasPrefix(s, " ") {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return s
}
