package wappalyzer

import (
	"testing"

	"github.com/killmonday/fscanx/mylib/finger/resources"
	"github.com/stretchr/testify/require"
)

// skipIfDataMissing 在引擎数据未嵌入时跳过(974150e 内存优化裁剪了 wappalyzer 数据 embed,
// 主程序仅启用 fingers+goby); 数据恢复嵌入后判据自动失效, 测试恢复真跑。
func skipIfDataMissing(t *testing.T) {
	if len(resources.WappalyzerData) == 0 {
		t.Skip("wappalyzer 引擎数据未嵌入(974150e 内存裁剪), 跳过")
	}
}

func TestCookiesDetect(t *testing.T) {
	skipIfDataMissing(t)
	wappalyzer, err := NewWappalyzeEngine()
	require.Nil(t, err, "could not create wappalyzer")

	matches := wappalyzer.Fingerprint(map[string][]string{
		"Set-Cookie": {"_uetsid=ABCDEF"},
	}, []byte(""))

	require.Contains(t, matches, "Microsoft Advertising", "Could not get correct match")

	t.Run("position", func(t *testing.T) {
		wappalyzerClient, _ := NewWappalyzeEngine()

		fingerprints := wappalyzerClient.Fingerprint(map[string][]string{
			"Set-Cookie": {"path=/; jsessionid=111; path=/, jsessionid=111;"},
		}, []byte(""))
		fingerprints1 := wappalyzerClient.Fingerprint(map[string][]string{
			"Set-Cookie": {"jsessionid=111; path=/, XSRF-TOKEN=; expires=test, path=/ laravel_session=eyJ*"},
		}, []byte(""))

		require.Equal(t, map[string]struct{}{"Java": {}}, fingerprints, "could not get correct fingerprints")
		require.Equal(t, map[string]struct{}{"Java": {}, "Laravel": {}, "PHP": {}}, fingerprints1, "could not get correct fingerprints")
	})
}

func TestHeadersDetect(t *testing.T) {
	skipIfDataMissing(t)
	wappalyzer, err := NewWappalyzeEngine()
	require.Nil(t, err, "could not create wappalyzer")

	matches := wappalyzer.Fingerprint(map[string][]string{
		"Server": {"now"},
	}, []byte(""))

	require.Contains(t, matches, "Vercel", "Could not get correct match")
}

func TestBodyDetect(t *testing.T) {
	skipIfDataMissing(t)
	wappalyzer, err := NewWappalyzeEngine()
	require.Nil(t, err, "could not create wappalyzer")

	t.Run("meta", func(t *testing.T) {
		matches := wappalyzer.Fingerprint(map[string][]string{}, []byte(`<html>
<head>
<meta name="generator" content="mura cms 1">
</head>
</html>`))
		require.Contains(t, matches, "Mura CMS:1", "Could not get correct match")
	})

	t.Run("html-implied", func(t *testing.T) {
		matches := wappalyzer.Fingerprint(map[string][]string{}, []byte(`<html data-ng-app="rbschangeapp">
<head>
</head>
<body>
</body>
</html>`))
		require.Contains(t, matches, "AngularJS", "Could not get correct implied match")
		require.Contains(t, matches, "PHP", "Could not get correct implied match")
		require.Contains(t, matches, "Proximis Unified Commerce", "Could not get correct match")
	})
}
