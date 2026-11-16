package main

// fscanx-web 登录认证（BL-POC-007）：默认账号 admin/admin123，登录签发内存会话 token，
// /api/*（除登录端点）凭 Authorization 头校验，无效返回 200+{code:401,message} 信封——
// 前端拦截器对 code:401 自动登出回登录页，会话过期链路零前端改动。
// 本地单用户维护端：凭据常量、token 表随进程生命周期（重启失效）、无防爆破限速（见 item 不做节）。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// 默认凭据（改密码=改常量重编译；本地单用户工具语义）。
const (
	authUsername = "admin"
	authPassword = "admin123"
)

var authSessions = struct {
	sync.Mutex
	m map[string]bool
}{m: make(map[string]bool)}

// newSessionToken 签发 32 字节随机会话 token（crypto/rand 失败属熵源异常，认证材料不降级直接 panic）。
func newSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func validSession(token string) bool {
	authSessions.Lock()
	defer authSessions.Unlock()
	return authSessions.m[token]
}

func dropSession(token string) {
	authSessions.Lock()
	defer authSessions.Unlock()
	delete(authSessions.m, token)
}

// bearerToken 从 Authorization 头取 token（接受裸 token 与 Bearer 前缀——前端拦截器发裸 token）。
func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	return strings.TrimPrefix(h, "Bearer ")
}

// userLogin POST /api/user/login {username,password} → {accessToken, user:{username}}。
// 响应形态与前端 LoginResult 契约一致（setSession 直接消费）。
func (s *server) userLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Username != authUsername || req.Password != authPassword {
		httpErrorf(w, "用户名或密码错误")
		return
	}
	token := newSessionToken()
	authSessions.Lock()
	authSessions.m[token] = true
	authSessions.Unlock()
	writeJSON(w, map[string]interface{}{
		"accessToken": token,
		"user":        map[string]string{"username": authUsername},
	})
}

// userLogout 注销当前 token（header 缺失/无效同样返回成功——登出语义幂等）。
func (s *server) userLogout(w http.ResponseWriter, r *http.Request) {
	if t := bearerToken(r); t != "" {
		dropSession(t)
	}
	writeJSON(w, map[string]string{"status": "logged_out"})
}

// authMiddleware /api/* 认证门（登录端点与静态资源放行）。
func (s *server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/user/login" {
			next.ServeHTTP(w, r)
			return
		}
		if !validSession(bearerToken(r)) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 401, "message": "未登录或会话已过期"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
