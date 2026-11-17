package main

// GET /api/routes 返回前端动态菜单（vue-element-plus-admin 动态路由契约）：
// component 'views/Xxx/Xxx' 映射 src/views 下组件；'#' 为 Layout 容器；
// meta.title 直写中文（i18n 未命中回退原文）。本地维护端无权限模型，
// 菜单随功能演进在后端此文件配置——未来新增页面在此追加一条即可。

import "net/http"

func (s *server) listRoutes(w http.ResponseWriter, r *http.Request) {
	meta := func(title, icon string) map[string]interface{} {
		return map[string]interface{}{"title": title, "icon": icon}
	}
	leaf := func(path, component, name string, m map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"path": path, "component": component, "name": name, "meta": m}
	}
	group := func(path, redirect, name string, m map[string]interface{}, children ...map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"path": path, "component": "#", "redirect": redirect, "name": name, "meta": m, "children": children}
	}
	writeJSON(w, []map[string]interface{}{
		group("/poc", "/poc/list", "Poc", meta("POC 管理", "mdi:bug"),
			leaf("list", "views/Poc/Poc", "PocList", meta("POC 管理", "mdi:bug"))),
		group("/catalog", "/catalog/list", "Catalog", meta("标准产品", "mdi:link-variant"),
			leaf("list", "views/Catalog/Catalog", "CatalogList", meta("标准产品", "mdi:link-variant"))),
		group("/finger", "/finger/edit", "Finger", meta("指纹库", "mdi:fingerprint"),
			leaf("edit", "views/Finger/Finger", "FingerEdit", meta("指纹库", "mdi:fingerprint"))),
	})
}
