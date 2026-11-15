package main

// 谓词树 ↔ CEL 表达式双向转换（BL-POC-004 poc 表单化编辑的技术核心）。
//
// 表单可生成的子集（与 PocScan/lib 求值环境的 Response 字段一一对应）：
//   status==N / body.bcontains / body.bmatches / headers["K"].contains，
//   条件间 && / || 任意嵌套（组带括号）。
// 反解析（CEL→树）遇到子集外节点（多步利用链、变量、其他函数）返回 nil，
// 前端收到 nil 即整卡切"高级源码模式"显示原文，不做不可靠的强解析。

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/killmonday/fscanx/PocScan/lib"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// Node 谓词树节点：组节点（Logic 非空，Conditions 为子项）或条件节点（Target 非空）。
type Node struct {
	Logic      string  `json:"logic,omitempty"`      // and | or（组节点）
	Conditions []*Node `json:"conditions,omitempty"` // 组节点子项
	Target     string  `json:"target,omitempty"`     // status | body | header（条件节点）
	Key        string  `json:"key,omitempty"`        // header 名（target=header 时必填）
	Op         string  `json:"op,omitempty"`         // eq | contains | regex
	Value      string  `json:"value,omitempty"`
}

func (n *Node) isGroup() bool { return n.Logic != "" }

// ToCEL 谓词树序列化为 CEL 表达式（组带括号）。
func (n *Node) ToCEL() (string, error) {
	if n.isGroup() {
		if n.Logic != "and" && n.Logic != "or" {
			return "", fmt.Errorf("逻辑连接词须为 and/or")
		}
		if len(n.Conditions) == 0 {
			return "", fmt.Errorf("条件组不能为空")
		}
		parts := make([]string, 0, len(n.Conditions))
		for i, c := range n.Conditions {
			if c == nil {
				return "", fmt.Errorf("第 %d 个条件为空", i+1)
			}
			s, err := c.ToCEL()
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		joiner := " && "
		if n.Logic == "or" {
			joiner = " || "
		}
		return "(" + strings.Join(parts, joiner) + ")", nil
	}
	return n.conditionCEL()
}

// conditionCEL 单条件序列化。
func (n *Node) conditionCEL() (string, error) {
	switch {
	case n.Target == "status" && n.Op == "eq":
		if _, err := strconv.Atoi(n.Value); err != nil {
			return "", fmt.Errorf("状态码须为整数: %q", n.Value)
		}
		return "response.status == " + n.Value, nil
	case n.Target == "body" && n.Op == "contains":
		return fmt.Sprintf("response.body.bcontains(b%q)", n.Value), nil
	case n.Target == "body" && n.Op == "regex":
		// bmatches 注册签名为 string receiver（lib/eval.go string_bmatches_bytes），
		// 内置 poc 生态统一形态："pattern".bmatches(response.body)（参数反转）
		return fmt.Sprintf("%q.bmatches(response.body)", n.Value), nil
	case n.Target == "header" && n.Op == "contains":
		if n.Key == "" {
			return "", fmt.Errorf("header 匹配须填 header 名")
		}
		return fmt.Sprintf("response.headers[%q].contains(%q)", n.Key, n.Value), nil
	default:
		return "", fmt.Errorf("不支持的匹配项/操作: %s/%s", n.Target, n.Op)
	}
}

// newCelEnv 构建与 poc 求值同源的 CEL 环境（编译校验与反解析共用）。
func newCelEnv() (*cel.Env, error) {
	c := lib.NewEnvOption()
	return lib.NewEnv(&c)
}

// ParsePredicate 将 CEL 表达式反解析为谓词树；编译失败或超出表单子集返回 nil。
func ParsePredicate(src string, env *cel.Env) *Node {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil
	}
	ast, iss := env.Parse(src)
	if iss.Err() != nil {
		return nil
	}
	return nodeFromExpr(ast.Expr())
}

// nodeFromExpr exprpb AST → 谓词树（子集白名单外返回 nil）。
func nodeFromExpr(e *exprpb.Expr) *Node {
	if e == nil {
		return nil
	}
	switch ex := e.ExprKind.(type) {
	case *exprpb.Expr_CallExpr:
		c := ex.CallExpr
		switch c.Function {
		case "_&&_", "_||_":
			if len(c.Args) != 2 {
				return nil
			}
			g := &Node{Logic: "and"}
			if c.Function == "_||_" {
				g.Logic = "or"
			}
			for _, a := range c.Args {
				sub := nodeFromExpr(a)
				if sub == nil {
					return nil
				}
				g.Conditions = append(g.Conditions, sub)
			}
			return g
		case "_==_":
			if len(c.Args) != 2 || selectPath(c.Args[0]) != "response.status" {
				return nil
			}
			v := constStringValue(c.Args[1])
			if v == "" {
				return nil
			}
			if _, err := strconv.Atoi(v); err != nil {
				return nil
			}
			return &Node{Target: "status", Op: "eq", Value: v}
		case "bcontains":
			recv, arg := callRecvArg(c)
			val := constStringValue(arg)
			if val == "" || selectPath(recv) != "response.body" {
				return nil
			}
			return &Node{Target: "body", Op: "contains", Value: val}
		case "bmatches":
			// 反转形态："pattern".bmatches(response.body)——receiver 是正则字面量
			recv, arg := callRecvArg(c)
			pattern := constStringValue(recv)
			if pattern == "" || selectPath(arg) != "response.body" {
				return nil
			}
			return &Node{Target: "body", Op: "regex", Value: pattern}
		case "contains":
			recv, arg := callRecvArg(c)
			val := constStringValue(arg)
			if val == "" {
				return nil
			}
			path := selectPath(recv)
			if strings.HasPrefix(path, "response.headers[") {
				key := strings.TrimSuffix(strings.TrimPrefix(path, "response.headers["), "]")
				key = strings.Trim(key, `"`)
				return &Node{Target: "header", Key: key, Op: "contains", Value: val}
			}
			return nil
		}
		return nil
	}
	return nil
}

// callRecvArg 兼容 member call（Target+Args）与 global call（全 Args）两种形态，
// 返回 receiver 与首个参数表达式（字面量与否由调用方按语义判断）。
func callRecvArg(c *exprpb.Expr_Call) (*exprpb.Expr, *exprpb.Expr) {
	if c.Target != nil && len(c.Args) == 1 {
		return c.Target, c.Args[0]
	}
	if c.Target == nil && len(c.Args) == 2 {
		return c.Args[0], c.Args[1]
	}
	return nil, nil
}

// selectPath 拼出 select/index 链的字符串路径（如 response.body、response.headers["K"]）；
// 非 select/index/ident 节点返回空串。
func selectPath(e *exprpb.Expr) string {
	if e == nil {
		return ""
	}
	switch ex := e.ExprKind.(type) {
	case *exprpb.Expr_IdentExpr:
		return ex.IdentExpr.Name
	case *exprpb.Expr_SelectExpr:
		base := selectPath(ex.SelectExpr.Operand)
		if base == "" {
			return ""
		}
		return base + "." + ex.SelectExpr.Field
	case *exprpb.Expr_CallExpr:
		c := ex.CallExpr
		if c.Function == "_[_]" && len(c.Args) == 2 {
			base := selectPath(c.Args[0])
			key := constStringValue(c.Args[1])
			if base == "" || key == "" {
				return ""
			}
			return base + "[" + strconv.Quote(key) + "]"
		}
	}
	return ""
}

// constStringValue 提取字面量节点的字符串值（bytes/string 原样、int 转十进制）；非字面量返回空。
func constStringValue(e *exprpb.Expr) string {
	if e == nil {
		return ""
	}
	if ce, ok := e.ExprKind.(*exprpb.Expr_ConstExpr); ok {
		switch v := ce.ConstExpr.ConstantKind.(type) {
		case *exprpb.Constant_StringValue:
			return v.StringValue
		case *exprpb.Constant_BytesValue:
			return string(v.BytesValue)
		case *exprpb.Constant_Int64Value:
			return strconv.FormatInt(v.Int64Value, 10)
		}
	}
	return ""
}
