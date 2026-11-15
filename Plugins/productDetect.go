package Plugins

import (
	"net/http"

	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
)

// detectProductsSilent 静默识别首页指纹（不拼行不输出），供 -m webpoc 模式 POC 智能定向（BL-POC-003）。
// 与 DoWebScan 同源：ReadRawWithSize 拼回完整 HTTP raw 报文（DetectContent 的输入契约——
// 引擎内部会做 ReadResponse，纯 body 会解析失败）；favicon 链兜底 favicon-only 产品
// （无文本特征的产品定向依赖此链，与 web 扫描链尾合并口径一致）。失败静默返回——
// 无指纹输入时智能定向自然不打（-pocname all 仍可全量）。
func detectProductsSilent(targetInfo *common.HostInfo) {
	req, err := http.NewRequest("GET", targetInfo.Url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-agent", common.UserAgent)
	resp, err := lib.ClientNoRedirect.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	raw, err := ReadRawWithSize(resp, 192*1024) // 读取上限与文本指纹路径一致（模块 02 约定）
	if err != nil && len(raw) == 0 {
		return
	}
	if frames, err := Engine.DetectContent(raw); err == nil {
		for name := range frames {
			targetInfo.Frames = append(targetInfo.Frames, name)
		}
	}
	if common.NoFavicon {
		return
	}
	if icon := detectSiteFavicon(targetInfo.Url, nil, nil); len(icon) > 0 {
		if frames := matchFaviconFrames(icon); len(frames) > 0 {
			for name := range frames {
				targetInfo.Frames = append(targetInfo.Frames, name)
			}
		}
	}
}
