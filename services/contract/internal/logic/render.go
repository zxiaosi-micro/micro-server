// 打印稿辅助（L2，FR-CTR-001）：模板变量填充 → Go 原生 PDF（ADR-18，进程内零外部依赖，无 Gotenberg）。
// 产出为「本地打印辅助稿」，非正式合同文书（基线=线下签署+上传归档，平台不解析文件内容）。
//
// PDF 生成采用最小自足实现：内嵌标准字体 Helvetica（WinAnsi 编码）+ 简单文本排版。
// 中文正文以 UTF-8 → 占位渲染保留（基础版式覆盖纯 ASCII 打印稿；CJK 字体内嵌为 L3 演进位，
// 届时替换 renderPDF 实现即可——切换点登记于 04 §2）。

package logic

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"micro-server/services/contract/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// RenderTemplateInternal 模板变量填充 → PDF（base64）。
// 缺必填变量 → errRenderVarsBad（变量定义来自 template.variables_json）。
func RenderTemplateInternal(l logx.Logger, tpl *model.ContractTemplate, varsJSON string) (string, error) {
	vars, err := parseVarValues(varsJSON)
	if err != nil {
		return "", errRenderVarsBad.WithCause(err)
	}
	// 必填变量校验
	for _, def := range parseVarDefs(nullStr(tpl.VariablesJson)) {
		if def.Required {
			if _, ok := vars[def.Key]; !ok {
				return "", errRenderVarsBad.WithMsg(fmt.Sprintf("缺少必填变量: %s(%s)", def.Key, def.Label))
			}
		}
	}
	body := fillVars(tpl.Body, vars)
	pdf := renderPDF("Contract "+tpl.Code, body)
	return base64Of(pdf), nil
}

// varDef 模板变量定义。
type varDef struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

func parseVarDefs(raw string) []varDef {
	var defs []varDef
	if raw == "" {
		return defs
	}
	_ = jsonUnmarshal(raw, &defs)
	return defs
}

func parseVarValues(raw string) (map[string]string, error) {
	out := map[string]string{}
	if raw == "" {
		return out, nil
	}
	err := jsonUnmarshal(raw, &out)
	return out, err
}

// fillVars {{key}} 占位填充（未提供变量保留占位便于发现）。
func fillVars(body string, vars map[string]string) string {
	return placeholdReplace(body, vars)
}

func placeholdReplace(s string, vars map[string]string) string {
	var sb strings.Builder
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			sb.WriteString(s)
			break
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			sb.WriteString(s)
			break
		}
		key := strings.TrimSpace(s[i+2 : i+j])
		sb.WriteString(s[:i])
		if v, ok := vars[key]; ok {
			sb.WriteString(v)
		} else {
			sb.WriteString("{{" + key + "}}")
		}
		s = s[i+j+2:]
	}
	return sb.String()
}

// jsonUnmarshal JSON 字符串反序列化辅助。
func jsonUnmarshal(raw string, v any) error {
	return json.Unmarshal([]byte(raw), v)
}

// ---- 最小 PDF 写出器（ADR-18：Go 原生进程内，零外部依赖）----

// renderPDF 生成简单 A4 文本 PDF（标题 + 正文换行排版；ASCII/WinAnsi）。
func renderPDF(title, body string) []byte {
	lines := wrapText(body, 92)
	if len(lines) > 58 {
		lines = lines[:58]
	}
	var objects []string

	content := "BT\n/F1 16 Tf\n72 780 Td\n(" + pdfEscape(title) + ") Tj\nET\n"
	content += "BT\n/F2 10 Tf\n72 756 Td\n(" + pdfEscape(time.Now().Format("2006-01-02 15:04")) + ") Tj\nET\n"
	y := 736.0
	for _, ln := range lines {
		content += fmt.Sprintf("BT\n/F2 10 Tf\n72 %.1f Td\n(%s) Tj\nET\n", y, pdfEscape(ln))
		y -= 14
	}

	objects = append(objects,
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
	)

	var pdf strings.Builder
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = pdf.Len()
		pdf.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, obj))
	}
	xref := pdf.Len()
	pdf.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1))
	for _, off := range offsets {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	pdf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref))
	return []byte(pdf.String())
}

// pdfEscape PDF 字符串转义 + 非 WinAnsi 字符降级（CJK 基线降级为 '?'——打印稿 L3 演进位替换字体内嵌）。
func pdfEscape(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString("\\\\")
		case '(':
			sb.WriteString("\\(")
		case ')':
			sb.WriteString("\\)")
		case '\r', '\n':
			sb.WriteString(" ")
		default:
			if r < 128 {
				sb.WriteRune(r)
			} else {
				sb.WriteByte('?') // CJK 降级位：L3 内嵌中文字体后消除
			}
		}
	}
	return sb.String()
}

// wrapText 简单按宽度换行（rune 计）。
func wrapText(s string, width int) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		for utf8.RuneCountInString(para) > width {
			cut := 0
			count := 0
			for idx, r := range para {
				if count == width {
					cut = idx
					break
				}
				count++
				cut = idx + utf8.RuneLen(r)
			}
			lines = append(lines, para[:cut])
			para = para[cut:]
		}
		lines = append(lines, para)
	}
	return lines
}

// base64Of 标准编码（传输层友好）。
func base64Of(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
