package Plugins

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chainreactors/utils/encode"
	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
)

// TestMain 初始化共享 http client（lib.Client 由 InitHttpClient 赋值，主程序启动时调用，
// 测试进程需要自行初始化，否则 fetchIconBytes 走 nil 防护静默放弃、网络用例全挂）
func TestMain(m *testing.M) {
	if err := lib.InitHttpClient("", 10*time.Second); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// TestExtractIconCandidates HTML 候选解析：内联/相对路径/base 基准/优先级排序/老写法/畸形 HTML
func TestExtractIconCandidates(t *testing.T) {
	pageURL, _ := url.Parse("http://example.com/a/b/index.html")

	cases := []struct {
		name      string
		html      string
		wantHref  string // 第一个候选（空表示无候选）
		wantInlne bool  // 是否解析出内联字节
	}{
		{
			name:      "base64 内联图标",
			html:      `<html><head><link rel="icon" href="data:image/png;base64,ZmFrZS1pY29uLWJ5dGVz"></head></html>`,
			wantInlne: true,
		},
		{
			name:      "非 base64 data-uri（svg 百分号编码）",
			html:      `<html><head><link rel="icon" href="data:image/svg+xml,%3Csvg%3Efake%3C/svg%3E"></head></html>`,
			wantInlne: true,
		},
		{
			name:     "相对路径解析为绝对 URL",
			html:     `<html><head><link rel="icon" href="icon.ico"></head></html>`,
			wantHref: "http://example.com/a/b/icon.ico",
		},
		{
			name:     "根相对路径",
			html:     `<html><head><link rel="shortcut icon" href="/static/img/fa.ico"></head></html>`,
			wantHref: "http://example.com/static/img/fa.ico",
		},
		{
			name:     "base href 作为相对基准",
			html:     `<html><head><base href="/app/"><link rel="icon" href="icon.ico"></head></html>`,
			wantHref: "http://example.com/app/icon.ico",
		},
		{
			name:     "主 rel 优先于 apple-touch-icon",
			html:     `<html><head><link rel="apple-touch-icon" href="/touch.png"><link rel="icon" href="/main.ico"></head></html>`,
			wantHref: "http://example.com/main.ico",
		},
		{
			name:     "HTML4 老写法：无 rel 但 .ico 结尾",
			html:     `<html><head><link href="/legacy.ico"></head></html>`,
			wantHref: "http://example.com/legacy.ico",
		},
		{
			name:     "非图标 link 不收集",
			html:     `<html><head><link rel="stylesheet" href="/style.css"></head></html>`,
			wantHref: "",
		},
		{
			name:     "畸形 HTML（截断+未闭合）不 panic",
			html:     "<html><head><link rel=\"icon\" href=\"/ok.ico\"><link rel=\"icon\" href=\"/bro",
			wantHref: "http://example.com/ok.ico",
		},
		{
			name:      "非图标 data URI 不占据内联名额",
			html:      `<html><head><link rel="stylesheet" href="data:text/css,body{}"><link rel="icon" href="data:image/png;base64,ZmFrZS1pY29uLWJ5dGVz"></head></html>`,
			wantInlne: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hrefs, inline := ExtractIconCandidates([]byte(c.html), pageURL)
			if c.wantHref == "" {
				if len(hrefs) != 0 {
					t.Fatalf("期望无候选，得到 %v", hrefs)
				}
			} else {
				if len(hrefs) == 0 || hrefs[0] != c.wantHref {
					t.Fatalf("期望首个候选 %s，得到 %v", c.wantHref, hrefs)
				}
			}
			if c.wantInlne && len(inline) == 0 {
				t.Fatalf("期望解析出内联图标字节，得到空")
			}
			if !c.wantInlne && len(inline) != 0 {
				t.Fatalf("不期望内联字节，得到 %d 字节", len(inline))
			}
		})
	}
}

// TestExtractIconCandidatesInlineContent 内联解码内容正确性（base64 与百分号两种形态）
func TestExtractIconCandidatesInlineContent(t *testing.T) {
	pageURL, _ := url.Parse("http://example.com/")

	_, inline := ExtractIconCandidates([]byte(
		`<link rel="icon" href="data:image/png;base64,ZmFrZS1pY29uLWJ5dGVz">`), pageURL)
	if string(inline) != "fake-icon-bytes" {
		t.Fatalf("base64 解码错误：%q", string(inline))
	}

	_, inline = ExtractIconCandidates([]byte(
		`<link rel="icon" href="data:image/svg+xml,%3Csvg%3Efake%3C/svg%3E">`), pageURL)
	if string(inline) != "<svg>fake</svg>" {
		t.Fatalf("百分号解码错误：%q", string(inline))
	}
}

// TestDetectSiteFavicon 三层调度：httptest 覆盖标准路径/link 优先/404/内联零请求
func TestDetectSiteFavicon(t *testing.T) {
	t.Run("标准路径兜底（首页无 link 声明）", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/favicon.ico" {
				_, _ = w.Write([]byte("std-icon"))
				return
			}
			_, _ = w.Write([]byte("<html>plain</html>"))
		}))
		defer srv.Close()
		pageURL, _ := url.Parse(srv.URL)
		hrefs, inline := ExtractIconCandidates([]byte("<html>plain</html>"), pageURL)
		if len(hrefs) != 0 || len(inline) != 0 {
			t.Fatalf("不期望候选，得到 hrefs=%v inline=%d", hrefs, len(inline))
		}
		got := detectSiteFavicon(srv.URL, hrefs, inline)
		if string(got) != "std-icon" {
			t.Fatalf("期望标准路径图标，得到 %q", got)
		}
	})

	t.Run("link 声明优先于标准路径（不同内容可分辨）", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/custom/theme.ico":
				_, _ = w.Write([]byte("custom-icon"))
			case "/favicon.ico":
				_, _ = w.Write([]byte("std-icon"))
			default:
				_, _ = w.Write([]byte(`<link rel="icon" href="/custom/theme.ico">`))
			}
		}))
		defer srv.Close()
		pageURL, _ := url.Parse(srv.URL)
		hrefs, inline := ExtractIconCandidates([]byte(`<link rel="icon" href="/custom/theme.ico">`), pageURL)
		got := detectSiteFavicon(srv.URL, hrefs, inline)
		if string(got) != "custom-icon" {
			t.Fatalf("期望 link 声明的图标，得到 %q", got)
		}
	})

	t.Run("404 与无候选返回 nil", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer srv.Close()
		if got := detectSiteFavicon(srv.URL, nil, nil); got != nil {
			t.Fatalf("期望 nil，得到 %q", got)
		}
	})

	t.Run("内联图标零请求（服务器不可达仍可得）", func(t *testing.T) {
		got := detectSiteFavicon("http://127.0.0.1:1/", nil, []byte("inline-icon"))
		if string(got) != "inline-icon" {
			t.Fatalf("期望内联图标，得到 %q", got)
		}
	})
}

// TestMatchAndMergeFavicon 哈希命中→alias 归并→行合并（含去重与空 products 衔接）
func TestMatchAndMergeFavicon(t *testing.T) {
	iconBytes := []byte("fake-icon-content")
	md5hash := encode.Md5Hash(iconBytes)

	// 向全局引擎注入测试哈希（测试结束恢复，fingers 生产表不含此哈希）。
	// 注意：写全局 map 依赖同包测试串行执行（go test 默认），任何用例不得引入 t.Parallel
	fav := Engine.Favicon()
	if fav == nil {
		t.Fatal("FaviconEngine 未初始化")
	}
	fav.Md5Fingers[md5hash] = "TestFaviconProduct"
	defer delete(fav.Md5Fingers, md5hash)

	frames := matchFaviconFrames(iconBytes)
	// Framework.String 输出形态为 [name]（Frameworks.Add 统一小写化名字）
	if frames.String() != "[testfaviconproduct]" {
		t.Fatalf("期望命中 [testfaviconproduct]，得到 %q", frames.String())
	}

	prefix := "[+] Product http://x/\t200\t(title)\t"
	t.Run("追加到已有指纹尾部", func(t *testing.T) {
		// 真实行形态：products 非空时 [Cert 段由 DoWebScan 拼为 ", [Cert:...]"
		got := mergeFaviconLine(prefix, "[php], [nginx]", ", [Cert:cn]", frames)
		want := "[+] Product http://x/\t200\t(title)\t[php], [nginx], [testfaviconproduct], [Cert:cn]"
		if got != want {
			t.Fatalf("合并错误：\n got %q\nwant %q", got, want)
		}
	})

	t.Run("与已有指纹去重", func(t *testing.T) {
		got := mergeFaviconLine(prefix, "[testfaviconproduct], [nginx]", "", frames)
		want := "[+] Product http://x/\t200\t(title)\t[testfaviconproduct], [nginx]"
		if got != want {
			t.Fatalf("去重错误：\n got %q\nwant %q", got, want)
		}
	})

	t.Run("products 为空时与后续段衔接", func(t *testing.T) {
		got := mergeFaviconLine(prefix, "", "[Cert:cn]\r\n", frames)
		want := "[+] Product http://x/\t200\t(title)\t[testfaviconproduct], [Cert:cn]\r\n"
		if got != want {
			t.Fatalf("衔接错误：\n got %q\nwant %q", got, want)
		}
	})

	t.Run("空 frames 原样返回（零差异底线）", func(t *testing.T) {
		got := mergeFaviconLine(prefix, "PHP", ", [ICP:x]", nil)
		want := prefix + "PHP" + ", [ICP:x]"
		if got != want {
			t.Fatalf("原样返回错误：\n got %q\nwant %q", got, want)
		}
	})

	t.Run("无后续段时不悬挂逗号（最常见行形态）", func(t *testing.T) {
		// suffix 仅含 "\r\n"（无 Cert/ICP 等段）时不得补 ", "，否则行尾悬挂 ", \r\n"
		got := mergeFaviconLine(prefix, "[php], [nginx]", "\r\n", frames)
		want := "[+] Product http://x/\t200\t(title)\t[php], [nginx], [testfaviconproduct]\r\n"
		if got != want {
			t.Fatalf("悬挂逗号或格式错误：\n got %q\nwant %q", got, want)
		}
	})
}

// TestFaviconEndToEnd 组合链：httptest 抓取 → 哈希注入命中 → 行合并格式（含无后续段形态）
// focus 注入断言 [*name] 形态（BL-FOC-005：favicon 哈希命中路径的 focus 传递）
func TestFaviconEndToEnd(t *testing.T) {
	iconBytes := []byte("e2e-icon-bytes")
	md5hash := encode.Md5Hash(iconBytes)
	fav := Engine.Favicon()
	fav.Md5Fingers[md5hash] = "E2EFavicon"
	fav.FocusNames["E2EFavicon"] = true
	defer func() {
		delete(fav.Md5Fingers, md5hash)
		delete(fav.FocusNames, "E2EFavicon")
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			_, _ = w.Write(iconBytes)
			return
		}
		_, _ = w.Write([]byte("<html><head><title>e2e</title></head><body>x</body></html>"))
	}))
	defer srv.Close()

	pageURL, _ := url.Parse(srv.URL)
	hrefs, inline := ExtractIconCandidates([]byte("<html>plain</html>"), pageURL)
	icon := detectSiteFavicon(srv.URL, hrefs, inline)
	if string(icon) != string(iconBytes) {
		t.Fatalf("期望抓到测试图标，得到 %q", icon)
	}
	frames := matchFaviconFrames(icon)
	if frames.String() != "[*e2efavicon]" {
		t.Fatalf("期望命中 focus 形态 [*e2efavicon]，得到 %q", frames.String())
	}
	prefix := "[+] Product " + srv.URL + "\t200\t(e2e)\t"
	line := mergeFaviconLine(prefix, "", "\r\n", frames)
	want := prefix + "[*e2efavicon]\r\n"
	if line != want || strings.Contains(line, ", \r\n") {
		t.Fatalf("端到端行格式错误：\n got %q\nwant %q", line, want)
	}
}

// TestDetectSiteFaviconBudget 请求预算：link 候选 404 后兜底标准路径仍受预算保护
func TestDetectSiteFaviconBudget(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		if r.URL.Path == "/dead.ico" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("std-icon"))
	}))
	defer srv.Close()

	// link 候选失败 → 兜底 /favicon.ico 成功，总请求数 <= iconMaxRequests
	got := detectSiteFavicon(srv.URL, []string{srv.URL + "/dead.ico"}, nil)
	if string(got) != "std-icon" {
		t.Fatalf("期望兜底成功，得到 %q", got)
	}
	if len(hits) > iconMaxRequests {
		t.Fatalf("请求数超预算：%v", hits)
	}
	if !strings.Contains(strings.Join(hits, ","), "/favicon.ico") {
		t.Fatalf("未尝试标准路径：%v", hits)
	}
}

// TestHandleScanLine 调度状态机：无行不动 pending / 链尾行（reurl 空）暂存 /
// 中间行（reurl 非空）即时输出且不覆盖 pending。中间行会向 stdout 输出一行（LogSuccess），测试噪声可接受。
// 注意行数据必须含 \t 分段：LogSuccess 输出管线对 "[+] Product" 行 Split("\t") 后无条件取
// slice[1]/slice[2]（log.go:81-95），无 \t 的行会越界 panic（生产行由 DoWebScan 的 Sprintf 保证 4 段）
func TestHandleScanLine(t *testing.T) {
	withLine := scanPresents{linePrefix: "[+] Product http://x/", lineSuffix: "\t200\t(t)\t[php]\r\n"}

	// 无行（DoWebScan err 路径返回零值）：pending 保持空
	var pending scanPresents
	handleScanLine(scanPresents{}, "", &pending)
	if pending.hasLine() {
		t.Fatal("无行不得写入 pending")
	}

	// 链尾行：暂存
	handleScanLine(withLine, "", &pending)
	if !pending.hasLine() || pending.line() != withLine.line() {
		t.Fatal("链尾行应暂存 pending")
	}

	// 中间行：即时输出（下方一行 stdout），pending 不被覆盖——跳转链中先暂存后中间的
	// 组合不存在（暂存即链尾），此处验证的是防御性不变式：中间行永不污染 pending
	tail := pending
	handleScanLine(withLine, "http://next/", &pending)
	if pending.line() != tail.line() {
		t.Fatal("中间行不得覆盖 pending")
	}
}

// TestNoFaviconDisable 开关关闭时链尾行原样输出（零差异回归）
func TestNoFaviconDisable(t *testing.T) {
	common.NoFavicon = true
	defer func() { common.NoFavicon = false }()

	// 服务器只回 404（若仍发起请求则证明开关失效）
	var requested bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// emitFinalLineWithFavicon 不 panic、不请求；行拼接逻辑等价性由 mergeFaviconLine 用例覆盖
	p := scanPresents{
		linePrefix:   "[+] Product ",
		lineProducts: "http://x/",
		lineSuffix:   "\t200\t(t)\tPHP",
	}
	emitFinalLineWithFavicon(p, srv.URL, &common.HostInfo{})
	if requested {
		t.Fatal("NoFavicon 开启时不应发起任何图标请求")
	}
}
