package Plugins

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
	fpcommon "github.com/killmonday/fscanx/mylib/finger/common"
	"golang.org/x/net/html"
)

// favicon 探测预算与字节上限：每站点最多 2 个额外请求、图标内容只取前 64KB
//（favicon 实际几乎都小于 32KB，首页 192KB 上限是给 HTML 的，图标不需要）。
const (
	iconMaxBytes    = 64 * 1024
	iconMaxRequests = 2
)

// iconPrimaryRel 主图标 rel token：含它的复合值（shortcut icon / alternate icon）
// 经 token 精确命中覆盖，无需单独枚举。
const iconPrimaryRel = "icon"

// iconSecondaryRels 次级图标 rel token：非主 favicon 形态，仅排主候选之后。
var iconSecondaryRels = map[string]bool{
	"apple-touch-icon":             true,
	"apple-touch-icon-precomposed": true,
	"mask-icon":                    true,
	"fluid-icon":                   true,
}

// ExtractIconCandidates 从页面原始响应（状态行+头+body 的 raw，与首页 body 同源）
// 解析 favicon 候选：返回按优先级排序的候选绝对 URL 列表（主 rel 在前），
// 以及 data URI 内联图标的解码字节（零请求成本，优先级最高）。解析失败静默返回空。
func ExtractIconCandidates(rawHTML []byte, pageURL *url.URL) (hrefs []string, inline []byte) {
	links := extractLinks(rawHTML)

	// 先扫 <base href>：相对 URL 的解析基准（不规范页面 base 可能出现在 link 之后）
	base := pageURL
	for _, v := range links {
		if v.isBase && v.href != "" {
			if b, err := url.Parse(v.href); err == nil {
				base = pageURL.ResolveReference(b)
				break
			}
		}
	}

	seen := make(map[string]bool)
	var primary, secondary []string
	for _, v := range links {
		if v.href == "" || v.isBase {
			continue
		}
		// data URI 内联图标：仅限声明为图标的 rel（浏览器语义），防止 stylesheet/font 等
		// 其他内联资源占据唯一名额阻断真 favicon 探测；解码成功即采纳，只取第一个
		if (v.relPrimary || v.relSecondary) && inline == nil && strings.HasPrefix(strings.ToLower(v.href), "data:") {
			if data := decodeDataURI(v.href); len(data) > 0 {
				inline = data
			}
			continue
		}
		// HTML4 老写法：无 rel 但 href 以 .ico 结尾也视为候选
		isIcon := v.relPrimary || v.relSecondary ||
			(v.rel == "" && strings.HasSuffix(strings.ToLower(v.href), ".ico"))
		if !isIcon {
			continue
		}
		u, err := base.Parse(v.href)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		if seen[u.String()] {
			continue
		}
		seen[u.String()] = true
		if v.relPrimary {
			primary = append(primary, u.String())
		} else {
			secondary = append(secondary, u.String())
		}
	}
	return append(primary, secondary...), inline
}

// iconLink 单个 <link>/<base> 标签的解析结果
type iconLink struct {
	href         string
	rel          string
	relPrimary   bool
	relSecondary bool
	isBase       bool
}

// extractLinks 用 html.Tokenizer 提取全部 <link> 标签的 href/rel 与 <base> 声明。
// 对畸形 HTML 容错（x/net/html 的强项），出错即返回已解析部分。
func extractLinks(rawHTML []byte) []iconLink {
	var links []iconLink
	z := html.NewTokenizer(bytes.NewReader(rawHTML))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return links
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if !strings.EqualFold(token.Data, "link") && !strings.EqualFold(token.Data, "base") {
			continue
		}
		item := iconLink{isBase: strings.EqualFold(token.Data, "base")}
		for _, attr := range token.Attr {
			switch strings.ToLower(attr.Key) {
			case "href":
				item.href = strings.TrimSpace(attr.Val)
			case "rel":
				item.rel = strings.ToLower(strings.TrimSpace(attr.Val))
			}
		}
		for _, tok := range strings.Fields(item.rel) {
			if tok == iconPrimaryRel {
				item.relPrimary = true
			}
			if iconSecondaryRels[tok] {
				item.relSecondary = true
			}
		}
		links = append(links, item)
	}
}

// decodeDataURI 解码 data URI 形式的内联图标：;base64 后缀走 base64 解码
//（含无 padding 容错，失败返回 nil），其余按百分号解码（失败原样返回——
// 畸形字节的哈希不在表内不会误报，交给查表自然淘汰）。
func decodeDataURI(href string) []byte {
	comma := strings.Index(href, ",")
	if comma == -1 {
		return nil
	}
	meta, data := strings.ToLower(href[:comma]), href[comma+1:]
	if strings.Contains(meta, ";base64") {
		if b, err := base64.StdEncoding.DecodeString(data); err == nil {
			return b
		}
		if b, err := base64.RawStdEncoding.DecodeString(data); err == nil {
			return b
		}
		return nil
	}
	if unescaped, err := url.PathUnescape(data); err == nil {
		return []byte(unescaped)
	}
	return []byte(data)
}

// fetchIconBytes 请求单个图标 URL，返回图标纯字节。非 200 / 超时 / 空内容一律返回 nil
//（favicon 是增强信息，失败静默放弃）。Connection: close 与 DoWebScan 同策略。
func fetchIconBytes(u string) []byte {
	// lib.Client 由 InitHttpClient 在主程序启动时初始化；未初始化场景（测试/异常路径）静默放弃
	if lib.Client == nil {
		return nil
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-agent", common.UserAgent)
	req.Header.Set("Connection", "close")
	resp, err := lib.Client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, iconMaxBytes))
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// detectSiteFavicon 三层候选源调度，预算 iconMaxRequests 个请求内拿到图标字节即停：
// 第 0 层 data URI 内联（零请求）→ 第 1 层 link 声明路径（站点自声明优先，浏览器语义）
// → 第 2 层 /favicon.ico 兜底。全部失败返回 nil。
func detectSiteFavicon(finalURL string, iconHrefs []string, iconInline []byte) []byte {
	if len(iconInline) > 0 {
		return iconInline
	}
	budget := iconMaxRequests
	// 第 1 层：link 声明的主候选（hrefs 已按主 rel 优先排序，只取第一个）
	if len(iconHrefs) > 0 && budget > 0 {
		budget--
		if data := fetchIconBytes(iconHrefs[0]); data != nil {
			return data
		}
	}
	// 第 2 层：标准路径兜底
	if budget > 0 {
		if u, err := url.Parse(finalURL); err == nil && u.Scheme != "" && u.Host != "" {
			u.Path = "/favicon.ico"
			u.RawQuery = ""
			u.Fragment = ""
			if data := fetchIconBytes(u.String()); data != nil {
				return data
			}
		}
	}
	return nil
}

// matchFaviconFrames 对图标字节算 md5+mmh3 查 FaviconEngine 哈希表，
// 再经 MergeFrameworks 做 alias 归并（fingers/goby 同产品异名统一）。
func matchFaviconFrames(iconBytes []byte) fpcommon.Frameworks {
	frames := Engine.MatchFavicon(iconBytes)
	return Engine.MergeFrameworks(make(fpcommon.Frameworks), frames)
}

// mergeFaviconLine 把 favicon 命中的指纹名合并进 Product 行（三段拼接形态）。
// products 为已有指纹串（", " 分隔），suffix 是 [Cert:...] 等后续段；
// 与已有指纹分词精确去重，无新增名时原样返回（零差异底线）。
func mergeFaviconLine(prefix, products, suffix string, frames fpcommon.Frameworks) string {
	names := nonEmptyProducts(strings.Split(frames.String(), "||"))
	if len(names) == 0 {
		return prefix + products + suffix
	}
	existing := nonEmptyProducts(strings.Split(products, ", "))
	var fresh []string
	for _, n := range names {
		dup := false
		for _, e := range existing {
			if e == n {
				dup = true
				break
			}
		}
		if !dup {
			fresh = append(fresh, n)
		}
	}
	if len(fresh) == 0 {
		return prefix + products + suffix
	}
	all := make([]string, 0, len(existing)+len(fresh))
	all = append(all, existing...)
	all = append(all, fresh...)
	merged := strings.Join(all, ", ")
	// products 原为空且存在后续段（[Cert:...] 等）时需补 ", " 衔接；suffix 至少含 "\r\n"
	// 不算后续段，不能补（否则行尾悬挂 ", "）。products 非空时后续段已带 ", " 前缀。
	if strings.HasPrefix(suffix, "[") {
		suffix = ", " + suffix
	}
	return prefix + merged + suffix
}

// emitFinalLineWithFavicon 链尾行统一出口：favicon 探测开启且命中时先合并指纹再输出，
// 关闭/失败/未命中时原样输出（与无本功能时逐字节一致）。无行（pending 为空）时 no-op。
func emitFinalLineWithFavicon(p scanPresents, finalURL string) {
	if !p.hasLine() {
		return
	}
	line := p.linePrefix + p.lineProducts + p.lineSuffix
	if !common.NoFavicon && finalURL != "" {
		if icon := detectSiteFavicon(finalURL, p.iconHrefs, p.iconInline); len(icon) > 0 {
			if frames := matchFaviconFrames(icon); len(frames) > 0 {
				line = mergeFaviconLine(p.linePrefix, p.lineProducts, p.lineSuffix, frames)
			}
		}
	}
	common.LogSuccess(line)
}
