package main

// 登录认证单测（BL-POC-007）：登录成功/失败、中间件放行与拦截、白名单、登出失效。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postJSON(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/user/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestUserLogin_Success(t *testing.T) {
	s := &server{}
	rec := postJSON(t, s.userLogin, `{"username":"admin","password":"admin123"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"accessToken"`) {
		t.Fatalf("正确凭据应签发 token: %d %s", rec.Code, rec.Body.String())
	}
	// 从响应提取 token 并校验会话有效
	var tokenStr string
	for _, line := range strings.Split(rec.Body.String(), `"`) {
		if len(line) == 64 { // hex 32 字节
			tokenStr = line
			break
		}
	}
	if tokenStr == "" || !validSession(tokenStr) {
		t.Fatalf("签发的 token 应在会话表: %q", tokenStr)
	}
}

func TestUserLogin_WrongCredentials(t *testing.T) {
	s := &server{}
	rec := postJSON(t, s.userLogin, `{"username":"admin","password":"wrong"}`)
	if !strings.Contains(rec.Body.String(), `"code":1`) || !strings.Contains(rec.Body.String(), "用户名或密码错误") {
		t.Fatalf("错误凭据应返回 code:1 错误信封: %s", rec.Body.String())
	}
}

func TestUserLogin_BadContentType(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest("POST", "/api/user/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.userLogin(rec, req)
	if !strings.Contains(rec.Body.String(), "Content-Type") {
		t.Fatalf("非 JSON Content-Type 应被拒: %s", rec.Body.String())
	}
}

func TestUserLogout_InvalidatesToken(t *testing.T) {
	s := &server{}
	rec := postJSON(t, s.userLogin, `{"username":"admin","password":"admin123"}`)
	var tokenStr string
	for _, line := range strings.Split(rec.Body.String(), `"`) {
		if len(line) == 64 {
			tokenStr = line
			break
		}
	}
	if tokenStr == "" || !validSession(tokenStr) {
		t.Fatalf("前置登录失败")
	}
	// logout 后同 token 失效
	req := httptest.NewRequest("GET", "/api/user/logout", nil)
	req.Header.Set("Authorization", tokenStr)
	recOut := httptest.NewRecorder()
	s.userLogout(recOut, req)
	if validSession(tokenStr) {
		t.Fatal("logout 后 token 应失效")
	}
}

// stubNext 中间件穿透标记。
func stubNext(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }

func TestAuthMiddleware_BlocksUnauthenticated(t *testing.T) {
	s := &server{}
	h := s.authMiddleware(http.HandlerFunc(stubNext))
	req := httptest.NewRequest("GET", "/api/binds_list", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":401`) {
		t.Fatalf("未认证 API 应返回 401 信封: %s", rec.Body.String())
	}
}

func TestAuthMiddleware_AllowsValidToken(t *testing.T) {
	s := &server{}
	authSessions.Lock()
	token := newSessionToken()
	authSessions.m[token] = true
	authSessions.Unlock()
	h := s.authMiddleware(http.HandlerFunc(stubNext))
	req := httptest.NewRequest("GET", "/api/binds_list", nil)
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "401") {
		t.Fatalf("有效 token 应放行: %d %s", rec.Code, rec.Body.String())
	}
	// Bearer 前缀同样接受
	req2 := httptest.NewRequest("GET", "/api/binds_list", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("Bearer 前缀应放行: %d", rec2.Code)
	}
}

func TestAuthMiddleware_Whitelist(t *testing.T) {
	s := &server{}
	h := s.authMiddleware(http.HandlerFunc(stubNext))
	// 登录端点放行
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/user/login", nil))
	if strings.Contains(rec.Body.String(), "401") {
		t.Fatal("登录端点应放行")
	}
	// 静态资源放行（登录页依赖）
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(rec2.Body.String(), "401") {
		t.Fatal("静态资源应放行")
	}
}
