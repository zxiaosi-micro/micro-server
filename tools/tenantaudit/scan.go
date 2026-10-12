// scan.go · model 源文件 AST 扫描：registry 登记提取 + SQL 轻量求值（S11-01）。
//
// 求值器覆盖 model 层常见写法：字符串字面量、+ 拼接、包级/函数级 var 常量传播、
// fmt.Sprintf 模板；不可解析成分（函数调用等）以 ⟨…⟩ 占位——不影响 tenant_id
// token 判定（tenant_id 只会以独立标识符出现在可求值段）。生成物 *_gen.go 跳过
// （架构红线禁止手改，且其表名/列名经变量拼接不可静态判定；custom 覆写层是审计面）。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// sqlEntry 一条被求值出的 SQL（或 SQL 片段）及其位置。
type sqlEntry struct {
	Text string
	File string
	Line int // 表达式起始行
}

// regEntry 一条 Scoped/Exempt 登记。
type regEntry struct {
	Table  string
	Kind   string // scoped | exempt
	Reason string
	File   string
	Line   int
}

// ignored 提取到的豁免行注释：行号 → 原因（tenantaudit:ignore <原因>）。
type fileScan struct {
	SQL       []sqlEntry
	Regs      []regEntry
	IgnoreAt  map[int]string // 行号 → 原因
	FullName  string
	ShortName string // 服务名（services/<svc> 的 <svc>）
}

var sqlKeyword = regexp.MustCompile(`(?i)\b(select\b|insert\s+into\b|update\b|delete\s+from\b|replace\s+into\b)`)

// scanModelFile 解析单个 model 源文件。
func scanModelFile(path, svc string) (*fileScan, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	fs := &fileScan{IgnoreAt: map[int]string{}, ShortName: svc, FullName: path}

	// 豁免注释（comment 可能挂在下一节点上，按行号全文收集）
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if pos := strings.Index(c.Text, "tenantaudit:ignore"); pos >= 0 {
				line := fset.Position(c.Pos()).Line
				reason := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.Text[pos+len("tenantaudit:ignore"):]), "//"))
				fs.IgnoreAt[line] = reason
			}
		}
	}

	// 包级 var 字符串（orderRows / xxSoftFilter 等）
	pkgVars := map[string]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, sp := range gd.Specs {
			vs := sp.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if i < len(vs.Values) {
					if s, ok := evalString(vs.Values[i], pkgVars, nil); ok {
						pkgVars[name.Name] = s
					}
				}
			}
		}
	}

	// registry 登记（tenantx.Scoped / tenantx.Exempt，init 或任意位置）
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !isTenantxPkg(sel.X) {
			return true
		}
		kind := ""
		switch sel.Sel.Name {
		case "Scoped":
			kind = "scoped"
		case "Exempt":
			kind = "exempt"
		default:
			return true
		}
		tbl, ok1 := litString(call.Args[0])
		if !ok1 {
			return true
		}
		reason := ""
		if kind == "exempt" {
			if len(call.Args) < 2 {
				return true
			}
			reason, _ = litString(call.Args[1])
		}
		pos := fset.Position(call.Pos())
		fs.Regs = append(fs.Regs, regEntry{Table: tbl, Kind: kind, Reason: reason, File: path, Line: pos.Line})
		return true
	})

	// 函数体 SQL：轻量常量传播后求值
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		localVars := map[string]string{}
		walkStmts(fd.Body.List, pkgVars, localVars, fs, path, fset)
	}
	return fs, nil
}

// collectSQL 遍历语句块：先按顺序传播局部 var 字符串赋值，再对每条语句里的
// 字符串表达式尝试 SQL 判定（赋值语句本身也产出 SQL，如 query := "..."）。
func collectSQL(stmt ast.Stmt, pkgVars, localVars map[string]string, fs *fileScan, path string, fset *token.FileSet) {
	// 先注册本语句定义的 var（供同语句内及后续引用），再扫 SQL。
	switch st := stmt.(type) {
	case *ast.AssignStmt:
		for i, lhs := range st.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || i >= len(st.Rhs) {
				continue
			}
			s, ok := evalString(st.Rhs[i], pkgVars, localVars)
			if !ok {
				continue
			}
			if st.Tok == token.ADD_ASSIGN { // where += " and ..."：追加而非覆盖
				if prev, has := localVars[id.Name]; has {
					localVars[id.Name] = prev + s
				}
				continue
			}
			localVars[id.Name] = s
		}
	case *ast.DeclStmt:
		if gd, ok := st.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i < len(vs.Values) {
						if s, ok := evalString(vs.Values[i], pkgVars, localVars); ok {
							localVars[name.Name] = s
						}
					}
				}
			}
		}
	}
	ast.Inspect(stmt, func(n ast.Node) bool {
		if n == stmt {
			return true
		}
		switch n.(type) {
		case *ast.FuncLit, *ast.BlockStmt:
			return false // 嵌套作用域不跨传播（model 层无此写法，保守跳过）
		}
		// 只对字符串表达式判定
		if be, ok := n.(*ast.BinaryExpr); ok && be.Op == token.ADD {
			if s, ok := evalString(be, pkgVars, localVars); ok && sqlKeyword.MatchString(s) {
				pos := fset.Position(be.Pos())
				fs.SQL = append(fs.SQL, sqlEntry{Text: s, File: path, Line: pos.Line})
			}
			return false // 已在拼接根处理
		}
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, ok := unquote(lit.Value); ok && sqlKeyword.MatchString(s) {
				pos := fset.Position(lit.Pos())
				fs.SQL = append(fs.SQL, sqlEntry{Text: s, File: path, Line: pos.Line})
			}
		}
		if ce, ok := n.(*ast.CallExpr); ok {
			if isSprintf(ce) && len(ce.Args) > 0 {
				if s, ok := evalString(ce, pkgVars, localVars); ok && sqlKeyword.MatchString(s) {
					pos := fset.Position(ce.Pos())
					fs.SQL = append(fs.SQL, sqlEntry{Text: s, File: path, Line: pos.Line})
				}
				return false // Sprintf 已按内联模板判定，实参不再重复
			}
			return true // 普通 CallExpr：深入找内层 Sprintf / 字面量实参（Exec(ctx, "insert…") 类）
		}
		return true
	})
	// 递归子语句（if/for/switch body）——SQL 主要在顶层，但保守起见扫一层块
	switch st := stmt.(type) {
	case *ast.IfStmt:
		if st.Body != nil {
			walkStmts(st.Body.List, pkgVars, localVars, fs, path, fset)
		}
	case *ast.ForStmt:
		if st.Body != nil {
			walkStmts(st.Body.List, pkgVars, localVars, fs, path, fset)
		}
	case *ast.RangeStmt:
		if st.Body != nil {
			walkStmts(st.Body.List, pkgVars, localVars, fs, path, fset)
		}
	case *ast.SwitchStmt:
		if st.Body != nil {
			for _, c := range st.Body.List {
				if cc, ok := c.(*ast.CaseClause); ok {
					walkStmts(cc.Body, pkgVars, localVars, fs, path, fset)
				}
			}
		}
	}
}

func walkStmts(list []ast.Stmt, pkgVars, localVars map[string]string, fs *fileScan, path string, fset *token.FileSet) {
	for _, s := range list {
		collectSQL(s, pkgVars, localVars, fs, path, fset)
	}
}

// evalString 轻量字符串求值：字面量/拼接/Ident（var 表）/fmt.Sprintf 模板。
// 其他表达式返回 ("⟨…⟩", true)——占位保持可拼接（tenant_id 判定不受影响）。
func evalString(e ast.Expr, pkgVars, localVars map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		return unquote(v.Value)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "⟨expr⟩", true
		}
		l, _ := evalString(v.X, pkgVars, localVars)
		r, _ := evalString(v.Y, pkgVars, localVars)
		return l + r, true
	case *ast.Ident:
		if s, ok := pkgVars[v.Name]; ok {
			return s, true
		}
		if s, ok := localVars[v.Name]; ok {
			return s, true
		}
		return "⟨" + v.Name + "⟩", true
	case *ast.CallExpr:
		if isSprintf(v) && len(v.Args) > 0 {
			if f, ok := evalString(v.Args[0], pkgVars, localVars); ok {
				return inlineSprintf(f, v.Args[1:], pkgVars, localVars), true
			}
		}
		return "⟨call⟩", true
	case *ast.ParenExpr:
		return evalString(v.X, pkgVars, localVars)
	}
	return "⟨expr⟩", true
}

func isSprintf(ce *ast.CallExpr) bool {
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "fmt"
}

// inlineSprintf 把 %s/%v/%d 实参按序内联进模板（ListPage 动态 where 判定的关键：
// tenant_id 位于 where 变量值里，不内联则模板恒缺租户条件）。
func inlineSprintf(format string, args []ast.Expr, pkgVars, localVars map[string]string) string {
	var b strings.Builder
	argIdx := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) || argIdx >= len(args) {
			b.WriteByte(format[i])
			continue
		}
		switch format[i+1] {
		case 's', 'v', 'd', 'q':
			s, _ := evalString(args[argIdx], pkgVars, localVars)
			b.WriteString(s)
			argIdx++
			i++
		case '%':
			b.WriteByte('%')
			i++
		default:
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

func isTenantxPkg(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "tenantx"
}

func litString(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		return "", false
	}
	return unquote(lit.Value)
}

func unquote(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1], true
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var out string
		if _, err := fmt.Sscanf(s, "%q", &out); err != nil {
			return s[1 : len(s)-1], true // 转义罕见，退化为去引号
		}
		return out, true
	}
	return "", false
}
