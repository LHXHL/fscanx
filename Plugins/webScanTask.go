package Plugins

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/killmonday/fscanx/PocScan"
	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
	fingers "github.com/killmonday/fscanx/mylib/finger"
	"github.com/killmonday/fscanx/mylib/stdio/chinese"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	regTitle       = regexp.MustCompile("(?ims)<title.*?>(.*?)</title>")
	regDirectInJS  = regexp.MustCompile(`(?i)location\.href\s*=\s*['"]([^'"]+)['"]`)
	regDirectInJS2 = regexp.MustCompile(`(?i)window\.navigate\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	regDirectInJS3 = regexp.MustCompile(`(?i)window\.location\.replace\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	regDirectInJS4 = regexp.MustCompile(`(?i)self\.location\s*=\s*['"]([^'"]+)['"]`)
	regDirectInJS5 = regexp.MustCompile(`(?i)top\.location\s*=\s*['"]([^'"]+)['"]`)
	regCopyRight   = regexp.MustCompile(`(?i)(.*)(©|&copy;|版权所有)(.*)`)
)

var Engine *fingers.Engine
var use_engine = []string{"fingers", "goby"} // fingers fingerprinthub wappalyzer ehole goby

func init() {
	// 创建finger引擎实例
	engineNew, err := fingers.NewEngine(use_engine...)
	if err != nil {
		common.Printf("Failed to create engine: %v\n", err)
		os.Exit(1)
	}
	Engine = engineNew
}

type stringer interface {
	String() string
}

var BufPool = sync.Pool{
	New: func() any {
		// 最大长度 192KB buffer
		return make([]byte, 192*1024)
	},
}

var BufBuildPool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

func ExtractTitleAndICP(htmlBytes []byte) (title string, icp string, copyright string, jumpUrl string) {
	// 从html字节流中提取 标题 和 ICP 备案号（基于html.Tokenizer）
	z := html.NewTokenizer(bytes.NewReader(htmlBytes))
	var inTitle bool
	var inScript bool
	var titleFound bool
	var jumpUrlFound bool
	var titleBuf strings.Builder
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			err := z.Err()
			if err == io.EOF {
				// 正常结束
				return
			}
			return
		}

		// 如果两个都找到了，提前终止，不再继续解析
		if title != "" && icp != "" && copyright != "" {
			return
		}

		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			tagName := token.Data // 如 "title"

			if !titleFound && strings.EqualFold(tagName, "title") {
				inTitle = true
				titleBuf.Reset() // 准备接收 title 文本
			} else if string(tagName) == "script" {
				inScript = true
			}

		case html.EndTagToken:
			token := z.Token()
			tagName := token.Data

			if !titleFound && strings.EqualFold(tagName, "title") {
				inTitle = false
				// 标题结束，提取已拼接的内容。还要做html实体转换，有一些标题全是html实体
				title = titleBuf.String()
				title = strings.ReplaceAll(title, "\n", "")
			}
			if inScript {
				inScript = false
			}

		case html.TextToken:
			//读取到可显文字
			text := strings.TrimSpace(string(z.Text()))
			text = chinese.ToUTF8(html.UnescapeString(text)) // 检测是中文编码才会转

			if text == "" {
				continue
			}
			if inTitle && text != "" {
				// 正在 title 标签内
				titleBuf.WriteString(text) // 检测是中文编码才会转utf-8
			}
			if !jumpUrlFound && inScript && text != "" {
				// 正在 script 标签内
				temp := getJSRedirectURL(z.Text())
				if temp != "" {
					jumpUrl = temp
					jumpUrlFound = true

				}
			}
			// 无论是否在 title 中，都尝试从当前文本节点中匹配 ICP 备案号
			if icp == "" && text != "" {
				if candidate := extractFirstICP(text); candidate != "" {
					icp = candidate
				}
			}
			// 版权信息
			match := regCopyRight.FindString(text)
			if match != "" {
				copyright = match
			}

		}
	}

}

func extractFirstICP(text string) string {
	// extractFirstICP 从一段文本中提取第一个看起来像 ICP 备案号的字符串
	// 支持如：京ICP备12345678号、沪ICP证000001号-1 等
	keywords := []string{"ICP备", "ICP证"} // 常见关键词，通常备案号中带有 "ICP备" 或 "ICP证"

	for _, kw := range keywords {
		idx := strings.Index(text, kw)
		if idx == -1 {
			continue
		}
		// 从 kw 开始，尝试提取后续的数字和“号”
		var rest string
		if idx-3 >= 0 {
			rest = text[idx-3:]
		} else {
			rest = text[idx:]
		}

		// 提取包含 kw 的连续片段，通常备案号格式比较规范
		candidate := extractICPCandidate(rest)
		if candidate != "" {
			return candidate
		}
	}

	return ""
}

func isChineseCharacter(s string) bool {
	// 将字符串的第一个字符转换为 rune 类型
	runeValue := []rune(s)[0]
	// 判断该字符是否属于中文字符范围
	return runeValue >= '\u4e00' && runeValue <= '\u9fa5'
}

func extractICPCandidate(s string) string {
	// 从疑似ICP备案号的text中提取icp
	// 启发式规则：找到 ICP备/ICP证 后，取后面 10~20 字符，通常包含数字和“号”
	start := strings.Index(s, "ICP")
	if start == -1 {
		return ""
	}
	// 从 ICP 开始，往后最多取 30 个字符，通常备案号不会太长
	end := start + 30
	if end > len(s) {
		end = len(s)
	}
	candidate := s[start:end]

	// 简单校验：是否含有数字和“号”字（常见于备案号结尾）
	if !strings.ContainsAny(candidate, "0123456789") {
		return ""
	}
	if !strings.Contains(candidate, "号") {
		return ""
	}

	// 做一个简单合理长度校验：ICP备+数字+号，总长度通常 10~20。这里是计算比较utf8字符串长度
	if utf8.RuneCountInString(candidate) < 10 || utf8.RuneCountInString(candidate) > 30 {
		return ""
	}

	// 正则匹配
	icpRegex := regexp.MustCompile(`(ICP备|ICP证)\d+[号\-]\d*号?`)
	matches := icpRegex.FindString(candidate)
	if matches != "" {
		if start-3 >= 0 {
			area := s[start-3 : start] //utf-8中，常见的中文占用3个字节
			if isChineseCharacter(area) {
				// 如果ICP前面是中文，大概率就是地区号
				return s[start-3:start] + matches
			}
		}
		return matches
	}
	if index := strings.Index(candidate, "ICP备案号："); index != -1 {
		if len(candidate) >= index+15 {
			return candidate[index+15:]
		}
		return candidate
	} else if index = strings.Index(candidate, "ICP备案号:"); index != -1 {
		if len(candidate) >= index+13 {
			return candidate[index+13:]
		}
		return candidate
	}
	return candidate
}

func WebScanController(targetInfo *common.HostInfo) error {
	common.WebScanRateCtrlCh <- struct{}{} // web探测任务的并发控制
	defer func() {
		<-common.WebScanRateCtrlCh
	}()
	if common.Scantype == "webpoc" {
		//仅PoC扫描。用户设置扫描类型是poc扫描，仅做poc扫描
		PocScan.PocScan(targetInfo)
		return nil
	}
	err := WebScanWorker(targetInfo) // web扫描

	if common.IsCheckPoc && err == nil {
		//用户启用了poc扫描（-poc），所以做完了web扫描就接着做poc扫描
		PocScan.PocScan(targetInfo)
	} else {
		// errlog := fmt.Sprintf("[-] WebScanController %v %v", targetInfo.Url, err)
		// common.LogError(errlog)
	}

	return err
}

func WebScanWorker(targetInfo *common.HostInfo) (err error) {
	//处理url并调用web扫描逻辑、web跳转控制及扫描等
	if targetInfo.Url == "" {
		// 如果url为空，说明输入是ip，此处构造url
		switch targetInfo.Ports {
		case "80":
			targetInfo.Url = fmt.Sprintf("http://%s", targetInfo.Host)
		case "443":
			targetInfo.Url = fmt.Sprintf("https://%s", targetInfo.Host)
		default:
			host := fmt.Sprintf("%s:%s", targetInfo.Host, targetInfo.Ports)
			protocol := IdentifyProtocol(host, common.TcpTimeout)
			targetInfo.Url = fmt.Sprintf("%s://%s:%s", protocol, targetInfo.Host, targetInfo.Ports)
		}
	} else {
		// url存在，说明输入的是域名资产，检查格式是否符合标准url，不符合就构造一下。否则不做任何操作
		if !strings.Contains(targetInfo.Url, "://") {
			host := strings.Split(targetInfo.Url, "/")[0]                     // 取出 host部分 拿去检测协议
			protocol := IdentifyProtocol(host, common.TcpTimeout)             //探测检查是http还是https
			targetInfo.Url = fmt.Sprintf("%s://%s", protocol, targetInfo.Url) //组装url
		}
	}
	// 发送http请求，识别web产品
	origUrl := targetInfo.Url // 目录扫描注册用原始入口 URL（下方跳转会改写 Url）
	seeds := &seedCollector{presents: make(map[string]struct{})}
	// pending 持链尾行（该次响应无后续跳转），链尾 favicon 探测合并后统一输出（BL-FP-005）
	var pending scanPresents
	err, reurl, ps := DoWebScan(targetInfo, 1, "")
	seeds.add(ps)
	if err != nil {
		return err
	}
	handleScanLine(ps, reurl, &pending)
	// 如果reurl是https，判断原始url是否就是https，如果不是则用https协议探测一遍
	if reurl == "https" {
		targetInfo.Url = strings.Replace(targetInfo.Url, "http://", "https://", 1)
		err, reurl, ps = DoWebScan(targetInfo, 1, "")
		seeds.add(ps)
		handleScanLine(ps, reurl, &pending)
		//还有跳转
		if strings.Contains(reurl, "://") {
			firstUrl := targetInfo.Url
			targetInfo.Url = reurl
			err, _, ps = DoWebScan(targetInfo, 3, firstUrl)
			seeds.add(ps)
			if err != nil {
				return
			}
			// scanType 3 只跟一层跳转，该行恒为链尾（返回的二次跳转不再跟随，维持原语义）
			handleScanLine(ps, "", &pending)
		}
	} else {
		//有url跳转
		if strings.Contains(reurl, "://") {
			firstUrl := targetInfo.Url
			targetInfo.Url = reurl
			err, _, ps = DoWebScan(targetInfo, 3, firstUrl)
			seeds.add(ps)
			if err != nil {
				return err
			}
			handleScanLine(ps, "", &pending)
		}
	}
	// 链尾行输出：favicon 探测开启且命中时合并指纹；targetInfo.Url 此刻为最终 URL
	emitFinalLineWithFavicon(pending, targetInfo.Url)
	if err == nil && seeds.anyBodyOK {
		// 目录扫描注册门槛：首页链至少一次响应 body 完整——err 不注册省 70 次
		// 注定失败的请求；body 全黑洞不注册省 70 次注定超时的请求（根路径都拿不到
		// 完整 body 的服务器，目录路径必然同样黑洞，用户洞察 2026-11-14。
		// 首页行输出不受影响：仅拦截注册，不动 DoWebScan 的输出路径）
		RegisterSite(origUrl, seeds.presents)
	}
	return err
}

// seedCollector 首页扫描链呈现收集器：呈现 key 并集（差集判定基准）+ body
// 健康位（目录扫描注册门槛），全链（初次/https重试/跳转）各次调用累积。
type seedCollector struct {
	presents  map[string]struct{}
	anyBodyOK bool
}

func (c *seedCollector) add(p scanPresents) {
	for _, k := range p.keys {
		c.presents[k] = struct{}{}
	}
	if p.bodyOK {
		c.anyBodyOK = true
	}
}

func parseCertField(value interface{}) string {
	var certByte = strings.Builder{}
	t := reflect.TypeOf(value)
	v := reflect.ValueOf(value)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		// 从0开始获取Student所包含的key
		key := t.Field(i)
		// 通过interface方法来获取key所对应的值
		value := v.Field(i).Interface()
		var cell string
		switch s := value.(type) {
		case string:
			cell = s
		case []string:
			cell = strings.Join(s, "; ")
		case int:
			cell = strconv.Itoa(s)
		case stringer:
			cell = s.String()
		}
		if cell == "" {
			continue
		}
		certByte.WriteString(key.Name)
		certByte.WriteString(": ")
		certByte.WriteString(cell)
		certByte.WriteString("\r\n")
	}
	return certByte.String()
}

func GetCertRaw(TLS *tls.ConnectionState) string {
	if TLS == nil {
		return ""
	}
	if len(TLS.PeerCertificates) == 0 {
		return ""
	}
	var certByte strings.Builder
	cert := TLS.PeerCertificates[0]
	certByte.WriteString(parseCertField(cert))
	certByte.WriteString("\r\n")
	certByte.WriteString("SUBJECT:\r\n")
	certByte.WriteString(parseCertField(cert.Subject))
	certByte.WriteString("\r\n")
	certByte.WriteString("Issuer:\r\n")
	certByte.WriteString(parseCertField(cert.Issuer))

	return certByte.String()
}

// scanPresents DoWebScan 的呈现输出：keys 为指纹非空时的呈现 key，
// bodyOK 标记本次响应 body 读取完整（根路径 body 黑洞的站点对目录扫描
// 是纯超时浪费——首页链健康位是目录扫描的注册门槛，BL-DIR-001）。
// linePrefix/lineProducts/lineSuffix 为 Product 行三段拆分（指纹区独立成段，
// 供链尾 favicon 指纹合并重拼，BL-FP-005）；iconHrefs/iconInline 为本页
// 解析出的 favicon 候选（零 IO，随行携带给链尾探测）。
type scanPresents struct {
	keys         []string
	bodyOK       bool
	linePrefix   string
	lineProducts string
	lineSuffix   string
	iconHrefs    []string
	iconInline   []byte
}

// hasLine 本响应是否拼出了 Product 行（err 路径不拼行）
func (p scanPresents) hasLine() bool {
	return p.linePrefix != "" || p.lineProducts != "" || p.lineSuffix != ""
}

// line 三段拼回完整行（无 favicon 合并时的原样输出形态）
func (p scanPresents) line() string {
	return p.linePrefix + p.lineProducts + p.lineSuffix
}

// handleScanLine 控制行输出时机（BL-FP-005）：中间行（reurl 非空，后面还有跳转）
// 即时输出——跳转源页面指纹不带 favicon；链尾行（reurl 为空）暂存 pending，
// 由 emitFinalLineWithFavicon 做 favicon 探测合并后统一输出。
func handleScanLine(ps scanPresents, reurl string, pending *scanPresents) {
	if !ps.hasLine() {
		return
	}
	if reurl != "" {
		common.LogSuccess(ps.line())
		return
	}
	*pending = ps
}

// DoWebScan 单次 Web 探测。第三返回值为本次响应的呈现（见 scanPresents，
// 供目录扫描差集判定与注册门槛使用，BL-DIR-001；不需要的调用方忽略即可）。
func DoWebScan(targetInfo *common.HostInfo, scanType int, firstUrl string) (error, string, scanPresents) {
	//scanType 1 初次访问，不进行跳转
	//scanType 3 访问一些跳转的URL，例如从初次访问得到的302、location等
	//scanType 4 初次访问响应400 -> 使用https重新扫描
	var client *http.Client
	var title string
	copyRight, icp, certInfo := "", "", ""
	var reurl string //提取到的跳转url
	var serverIp string

	req, err := http.NewRequest("GET", targetInfo.Url, nil)
	if err != nil {
		return err, "", scanPresents{}
	}
	req.Header.Set("User-agent", common.UserAgent)
	req.Header.Set("Accept", common.Accept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,en;q=0.8,zh;q=0.7,*")
	if common.Cookie != "" {
		req.Header.Set("Cookie", common.Cookie)
	}
	req.Header.Set("Connection", "close")
	// 创建Trace对象收集连接信息
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if common.Socks5Proxy == "" {
				addr := connInfo.Conn.RemoteAddr().String()
				if strings.HasPrefix(addr, "[") == false {
					index := strings.Index(addr, ":")
					serverIp = addr[:index]
				}
			}

		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	if scanType == 1 {
		client = lib.ClientNoRedirect
	} else {
		client = lib.Client
	}
	// 发送请求。不再对共享 client 赋值 Timeout：InitHttpClient 已用同一 WebTimeout 完成初始化，
	// 且并发写共享 client 的字段是数据竞争
	resp, err := client.Do(req)
	if resp != nil {
		defer func() {
			resp.Body.Close()
			req.Close = true
		}()
	}
	if err != nil {
		if !strings.HasPrefix(targetInfo.Url, "https") {
			return err, "https", scanPresents{} //如果报错，用https协议来重新探测一次
		}
		return err, "", scanPresents{}
	}

	body, err := ReadRawWithSize(resp, 192*1024) // 最大获取前192KB页面（2026-09-23 决策：保持192KB不变）
	var out scanPresents
	out.bodyOK = err == nil // body 读取完整性：超时/中断意味着响应头之后是黑洞，
	// 该信号经 WebScanWorker 作为目录扫描注册门槛（根路径都拿不到完整 body 的
	// 站点，目录路径必然同样超时）——err 本身维持既有忽略语义，不影响首页输出

	// 如果是https网站，获取证书信息
	if len(targetInfo.Url) >= 5 && targetInfo.Url[:5] == "https" {
		var certStr = strings.Builder{}
		certInfo = GetCertRaw(resp.TLS)
		subject := ""
		domainStr := "Domain="
		issuser := "Issuser="
		if certInfo != "" {
			// 提取subject
			if index := strings.Index(certInfo, "Subject"); index != -1 {
				if len(certInfo) >= index+9 {
					tmp := certInfo[index+9:]
					endIndex := strings.Index(tmp, "\r\n")
					if endIndex != -1 {
						subject += tmp[:endIndex]
					}
				}

			}
			// 提取域名
			if index := strings.Index(certInfo, "DNSNames"); index != -1 {
				tmp := certInfo[index+10:]
				endIndex := strings.Index(tmp, "\r\n")
				if endIndex != -1 {
					domainStr += tmp[:endIndex]
				}
			}

			// 提取颁发者信息
			if index := strings.Index(certInfo, "Issuer"); index != -1 {
				tmp := certInfo[index+9:]
				endIndex := strings.Index(tmp, "\r\n")
				if endIndex != -1 {
					issuser += tmp[:endIndex]
				}
			}
			var isFirst bool = true
			if len(subject) > 0 {
				certStr.WriteString(subject)
				isFirst = false
			}
			if len(domainStr) > 7 {
				if isFirst {
					certStr.WriteString(domainStr)
					isFirst = false
				} else {
					certStr.WriteString("," + domainStr)
				}
			}
			if len(issuser) > 8 {
				if isFirst {
					certStr.WriteString(issuser)
					isFirst = false
				} else {
					certStr.WriteString("," + issuser)
				}
			}

			certInfo = strings.ReplaceAll(certStr.String(), "\n", "_")
			certInfo = strings.ReplaceAll(certInfo, "\r", "_")
			/*
				cert_info content like this:

				SignatureAlgorithm: SHA256-RSA
				PublicKeyAlgorithm: RSA
				Version: 3
				SerialNumber: 4183520561172206966195670189914676379
				Issuer: CN=TrustAsia TLS RSA CA,OU=Domain Validated SSL,O=TrustAsia Technologies\, Inc.,C=CN
				Subject: CN=hbsd.top
				NotBefore: 2023-06-19 00:00:00 +0000 UTC
				NotAfter: 2024-06-18 23:59:59 +0000 UTC
				MaxPathLen: -1
				OCSPServer: http://statuse.digitalcertvalidation.com
				IssuingCertificateURL: http://cacerts.digitalcertvalidation.com/TrustAsiaTLSRSACA.crt
				DNSNames: hbsd.top; www.hbsd.top
			*/
		}
	}
	// 解析 favicon 候选（零 IO：body 已在内存，tokenizer 微秒级；候选随行携带，供链尾探测，BL-FP-005）
	if pageURL, perr := url.Parse(targetInfo.Url); perr == nil {
		out.iconHrefs, out.iconInline = ExtractIconCandidates(body, pageURL)
	}
	// 提取标题、备案号、版权信息
	title, icp, copyRight, reurl = ExtractTitleAndICP(body)
	title = normalizeTitle(title) // 与目录扫描 runDirTask 用同一规范化，保证呈现 key 一致
	if reurl != "" && !strings.Contains(reurl, "http") {
		// 解析 原始URL，准备组装跳转url
		parsedURL, err := url.Parse(targetInfo.Url)
		if err == nil {
			// 提取协议、主机名和端口（如果有）
			baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
			reurl = baseURL + "/" + reurl
		}
	}
	if copyRight != "" {
		copyRight = strings.Replace(copyRight, "版权所有", "", 1)
		copyRight = strings.Replace(copyRight, "Copyright", "", 1)
		copyRight = strings.Replace(copyRight, "：", "", 1)
		copyRight = strings.ReplaceAll(copyRight, "All Rights Reserved", "")
		copyRight = strings.Replace(copyRight, "备案号", "", 1)
		copyRight = strings.ReplaceAll(copyRight, "[", "")
		copyRight = strings.ReplaceAll(copyRight, "]", "")
		copyRight = strings.Replace(copyRight, "漏", "_", 1)
		if len(copyRight) > 69 {
			copyRight = copyRight[:69]
		}
	}
	//如果是初次扫描/跳转url扫描：进行资产识别、标题提取、跳转url提取等
	if reurl == "" {
		redirURL, err1 := resp.Location() // 通过http头部location尝试获取跳转
		if err1 == nil {
			reurl = redirURL.String()
			if reurl == targetInfo.Url { // || reurl == Url+"/" ，加了 / 有些时候确实页面不同
				reurl = ""
			}
		}
	}

	// 检测web指纹
	frames, err := Engine.DetectContent(body)
	products := frames.String()

	// 处理web识别的结果
	result := strings.Builder{}
	productSlice := strings.Split(products, "||")
	// 呈现 key（指纹非空才有）：供目录扫描差集判定；指纹排序规范化在 presentKey 内
	if fps := nonEmptyProducts(productSlice); len(fps) > 0 {
		out.keys = []string{presentKey(fps, title)}
	}
	result.WriteString("[+] Product ")
	result.WriteString(fmt.Sprintf("%s\t%d\t(%s)\t", targetInfo.Url, resp.StatusCode, title))
	productsOffset := result.Len() // 指纹区起点（链尾 favicon 指纹合并的插入基准，BL-FP-005）
	firstWriteFlag := false
	if products != "" && len(productSlice) >= 1 {
		result.WriteString(strings.Join(productSlice, ", "))
		firstWriteFlag = true
	}
	suffixOffset := result.Len() // 指纹区终点
	if certInfo != "" {
		if firstWriteFlag {
			result.WriteString(", [Cert:")
		} else {
			result.WriteString("[Cert:")
			firstWriteFlag = true
		}
		result.WriteString(certInfo)
		result.WriteString("]")
	}
	if firstUrl != "" {
		if scanType != 1 {
			if firstWriteFlag {
				result.WriteString(", [From:")
			} else {
				result.WriteString("[From:")
				firstWriteFlag = true
			}
			result.WriteString(firstUrl)
			result.WriteString("]")
		}

	}
	if icp != "" {
		if firstWriteFlag {
			result.WriteString(", [ICP:")
		} else {
			result.WriteString("[ICP:")
			firstWriteFlag = true
		}
		result.WriteString(icp)
		result.WriteString("]")
	}
	if copyRight != "" {
		if firstWriteFlag {
			result.WriteString(", [copyright:")
		} else {
			result.WriteString("[copyright:")
			firstWriteFlag = true
		}
		result.WriteString(copyRight)
		result.WriteString("]")
	}
	if serverIp != "" {
		if strings.HasPrefix(targetInfo.Url[7:], serverIp) || strings.HasPrefix(targetInfo.Url[8:], serverIp) {
		} else {
			if firstWriteFlag {
				result.WriteString(", [IP:")
			} else {
				result.WriteString("[IP:")
				firstWriteFlag = true
			}
			result.WriteString(serverIp)
			result.WriteString("]")
		}
		if Czdb != nil {
			ipInfo, err := Czdb.Find(serverIp)
			if err == nil {
				infoStr := ipInfo.String()
				if len(infoStr) != 0 && strings.HasPrefix(infoStr, "IANA|") == false {
					if firstWriteFlag {
						result.WriteString(", [L:")
					} else {
						result.WriteString("[L:")
						firstWriteFlag = true
					}
					result.WriteString(ipInfo.String())
					result.WriteString("]")
				}
			}
		}
	}

	result.WriteString("\r\n")
	// 行三段拆分存入 out，输出时机交调用方控制：中间行（还有跳转）即时输出，
	// 链尾行经 favicon 探测合并后输出（handleScanLine / emitFinalLineWithFavicon，BL-FP-005）
	full := result.String()
	out.linePrefix = full[:productsOffset]
	out.lineProducts = full[productsOffset:suffixOffset]
	out.lineSuffix = full[suffixOffset:]

	if reurl != "" {
		return nil, reurl, out
	}
	if resp.StatusCode == 400 && !strings.HasPrefix(targetInfo.Url, "https") {
		return nil, "https", out
	}
	return nil, "", out
}

// nonEmptyProducts 过滤指纹切片中的空串（products 为空时 Split 产生 [""]）。
func nonEmptyProducts(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// TimeoutReader 是一个包装了 io.Reader 的结构体，用于支持超时控制
type TimeoutReader struct {
	Reader io.Reader
	Ctx    context.Context
}

// Read 实现了 io.Reader 接口，并支持超时控制
func (tr *TimeoutReader) Read(p []byte) (n int, err error) {
	select {
	case <-tr.Ctx.Done(): // 如果超时，返回错误
		return 0, tr.Ctx.Err()
	default:
		return tr.Reader.Read(p)
	}
}

// 读取原始响应体，包括header+body
func ReadRawWithSize(resp *http.Response, size int64) ([]byte, error) {
	defer resp.Body.Close()
	//var raw bytes.Buffer
	raw := BufBuildPool.Get().(*bytes.Buffer)
	// defer 按 LIFO 执行：必须先 Reset 再 Put，保证 buffer 清理完毕才归还池，
	// 否则下一个取用者的写入会和本任务的 Reset 产生数据竞争
	defer BufBuildPool.Put(raw)
	defer raw.Reset()

	// http响应状态行
	//raw.WriteString(fmt.Sprintf("%s %s\r\n", resp.Proto, resp.Status))
	raw.WriteString(resp.Proto)
	raw.WriteString(" ")
	raw.WriteString(resp.Status)
	raw.WriteString("\r\n")

	// 响应头
	for k, v := range resp.Header {
		for _, val := range v {
			//raw.WriteString(fmt.Sprintf("%s: %s\r\n", k, val))
			raw.WriteString(k)
			raw.WriteString(": ")
			raw.WriteString(val)
			raw.WriteString("\r\n")
		}
	}
	raw.WriteString("\r\n") // header-body 分隔符

	// Body（仅前 size 字节）
	buf := BufPool.Get().([]byte)
	defer BufPool.Put(buf)

	// 创建带有超时控制的 context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(common.WebTimeout)*time.Second)
	defer cancel()

	limitedReader := io.LimitReader(resp.Body, size)
	timeoutReader := &TimeoutReader{
		Reader: limitedReader,
		Ctx:    ctx,
	}
	n, err := io.ReadFull(timeoutReader, buf)

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		err = nil // io.ErrUnexpectedEOF说明body 小于 192KB，如果是这个error则忽略
	} else if err != nil {
		// 池化 buffer 必须拷贝后再返回：raw 此时已归还 BufBuildPool，
		// 直接返回 raw.Bytes() 会与后续任务共享底层数组，造成池污染数据竞争
		return append([]byte(nil), raw.Bytes()...), err
	}

	raw.Write(buf[:n])

	// 如果启用了http keep-alive，且body未读完，则继续读取（丢弃）剩余的部分，这是为了能复用http连接，否则检测到body未读完会直接弃用该连接
	//if resp.Close == false {
	//	_, _ = io.Copy(io.Discard, resp.Body)
	//}

	// 返回完整的响应数据（必须拷贝，原因同上：池化 buffer 的底层内存不得逸出到池外）
	return append([]byte(nil), raw.Bytes()...), nil
}

func stripFirstChar(path string) string {
	if strings.HasPrefix(path, "/") {
		return path[1:]
	}
	return path
}

func getJSRedirectURL(body []byte) (redirectURL string) {
	// 匹配 location.href = 'target.jsp'
	find := regDirectInJS.FindSubmatch(body)
	if len(find) > 1 {
		redirectURL = string(find[1])
		redirectURL = strings.TrimSpace(redirectURL)
		redirectURL = strings.Replace(redirectURL, "&nbsp;", " ", -1)
		return stripFirstChar(redirectURL)
	}

	// 匹配 window.navigate('target.jsp')
	find = regDirectInJS2.FindSubmatch(body)
	if len(find) > 1 {
		redirectURL = string(find[1])
		redirectURL = strings.TrimSpace(redirectURL)
		redirectURL = strings.Replace(redirectURL, "&nbsp;", " ", -1)
		return stripFirstChar(redirectURL)
	}

	// 匹配 window.location.replace('target.jsp')
	find = regDirectInJS3.FindSubmatch(body)
	if len(find) > 1 {
		redirectURL = string(find[1])
		redirectURL = strings.TrimSpace(redirectURL)
		redirectURL = strings.Replace(redirectURL, "&nbsp;", " ", -1)
		return stripFirstChar(redirectURL)
	}

	// 匹配 self.location='target.aspx'
	find = regDirectInJS4.FindSubmatch(body)
	if len(find) > 1 {
		redirectURL = string(find[1])
		redirectURL = strings.TrimSpace(redirectURL)
		redirectURL = strings.Replace(redirectURL, "&nbsp;", " ", -1)
		return stripFirstChar(redirectURL)
	}

	// 匹配 top.location='target.aspx'
	find = regDirectInJS5.FindSubmatch(body)
	if len(find) > 1 {
		redirectURL = string(find[1])
		redirectURL = strings.TrimSpace(redirectURL)
		redirectURL = strings.Replace(redirectURL, "&nbsp;", " ", -1)
		return stripFirstChar(redirectURL)
	}

	// 没有找到任何Jump To
	redirectURL = ""
	return
}

func IdentifyProtocol(host string, Timeout int64) (protocol string) {
	protocol = "http"
	//如果端口是80或443,跳过Protocol判断
	if strings.HasSuffix(host, ":80") || !strings.Contains(host, ":") {
		return
	} else if strings.HasSuffix(host, ":443") {
		protocol = "https"
		return
	}

	socksconn, err := common.GetConn("tcp", host, time.Duration(Timeout)*time.Second)
	if err != nil {
		return
	}
	conn := tls.Client(socksconn, &tls.Config{MinVersion: tls.VersionTLS10, InsecureSkipVerify: true})
	defer func() {
		if conn != nil {
			defer func() {
				if err := recover(); err != nil {
					common.LogError(err)
				}
			}()
			conn.Close()
		}
	}()
	conn.SetDeadline(time.Now().Add(time.Duration(Timeout) * time.Second))
	err = conn.Handshake()
	if err == nil || strings.Contains(err.Error(), "handshake failure") {
		protocol = "https"
	}
	return protocol
}
