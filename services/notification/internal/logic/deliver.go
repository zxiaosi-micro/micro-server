// Deliver 内核：模板渲染 → 免打扰/频控过滤 → 站内信落库 → 渠道外发（outbound_log）。
// 事件消费（kq）与 RPC Deliver 共用本内核，保证过滤口径一致。

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"micro-server/services/notification/internal/model"
	"micro-server/services/notification/internal/provider"
	"micro-server/services/notification/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// deliverInput Deliver 内核入参（事件 payload 与 RPC 同构）。
type deliverInput struct {
	UserID       int64
	TemplateCode string
	Params       map[string]string
	BizType      string
	BizID        string
}

// deliverOutput 内核结果。
type deliverOutput struct {
	Delivered bool
	Reason    string
	MessageID int64
}

// rateLimitPer24h 频控上限（confcenter 热调兜底）。
func rateLimitPer24h(sc *svc.ServiceContext) int64 {
	if v := confRateLimit(); v > 0 {
		return v
	}
	return int64(sc.Config.RateLimitPer24h)
}

// providerFor 渠道 → Provider（转调 provider 包注册点）。
func providerFor(channel, providerCfg string) (provider.Provider, error) {
	return provider.For(channel, providerCfg)
}

// deliver 投递内核（tid 来自事件信封恢复或 RPC ctx）。
func deliver(ctx context.Context, sc *svc.ServiceContext, tid int64, in deliverInput) deliverOutput {
	// 1. 模板（缺失/停用 → 过滤）
	tpl, err := sc.Models.Template.FindByCode(ctx, tid, in.TemplateCode)
	if err != nil {
		if err == model.ErrNotFound {
			return deliverOutput{Reason: "template_not_found:" + in.TemplateCode}
		}
		return deliverOutput{Reason: "template_query_error"}
	}
	if tpl.Status != 1 {
		return deliverOutput{Reason: "template_disabled"}
	}

	// 2. 用户设置：静默开关（template_code 精确 > 全局 *）
	setting, serr := sc.Models.UserSetting.FindByUser(ctx, tid, in.UserID, in.TemplateCode)
	if serr == nil {
		for _, s := range setting {
			if s.TemplateCode == in.TemplateCode && s.Enabled == 0 {
				return deliverOutput{Reason: "user_muted:" + in.TemplateCode}
			}
		}
		// 全局默认静默（无精确设置时生效）
		hasExact := false
		for _, s := range setting {
			if s.TemplateCode == in.TemplateCode {
				hasExact = true
			}
		}
		if !hasExact {
			for _, s := range setting {
				if s.TemplateCode == "*" && s.Enabled == 0 {
					return deliverOutput{Reason: "user_muted:*"}
				}
			}
		}
	}

	// 3. 频控：同用户同模板 24h 内限 N 条（FR-NTF-002）
	bizKey := "tpl:" + in.TemplateCode
	if n, qerr := sc.Models.Message.CountByBiz24h(ctx, tid, in.UserID, bizKey); qerr == nil && n >= rateLimitPer24h(sc) {
		return deliverOutput{Reason: fmt.Sprintf("rate_limited:%d/24h", n)}
	}

	// 4. 渲染（{{key}} 占位替换）
	title := renderTemplate(tpl.TitleTemplate, in.Params)
	content := renderTemplate(tpl.ContentTemplate, in.Params)

	// 5. 站内信落库（INBOX 主通道）
	mid := sc.Snowflake.MustNextID()
	if _, ierr := sc.Models.Message.Insert(ctx, &model.Message{
		MessageId: mid,
		UserId:    in.UserID,
		Title:     title,
		Content:   content,
		IsRead:    0,
		BizType:   model.ToNullString(bizKey),
		BizId:     model.ToNullString(in.BizID),
		TenantId:  tid,
	}); ierr != nil {
		return deliverOutput{Reason: "inbox_write_error"}
	}

	// 6. 外发渠道（SMS 等 Provider；outbound_log 追加，失败不阻断站内信）
	if tpl.Channel != "" && tpl.Channel != "INBOX" {
		if p, perr := providerFor(tpl.Channel, channelProviderCfg(ctx, sc, tid, tpl.Channel)); perr == nil {
			oerr := p.Send(ctx, fmt.Sprintf("user:%d", in.UserID), title, content)
			if _, oerr2 := sc.Models.Outbound.Insert(ctx, &model.OutboundLog{
				OutboundId: sc.Snowflake.MustNextID(),
				MessageId:  model.ToNullInt64(mid),
				Channel:    tpl.Channel,
				Provider:   p.Name(),
				Target:     fmt.Sprintf("user:%d", in.UserID),
				Status:     statusOf(oerr == nil),
				Error:      model.ToNullString(errText(oerr)),
				TenantId:   tid,
			}); oerr2 != nil {
				logx.WithContext(ctx).Errorf("outbound_log 写入失败 message=%d: %v", mid, oerr2)
			}
		}
	}

	return deliverOutput{Delivered: true, MessageID: mid}
}

// renderTemplate {{key}} 占位替换（缺 key 留原样，便于排障）。
func renderTemplate(tpl string, params map[string]string) string {
	if len(params) == 0 {
		return tpl
	}
	r := tpl
	for k, v := range params {
		r = strings.ReplaceAll(r, "{{"+k+"}}", v)
	}
	return r
}

// channelProviderCfg 渠道 Provider 配置（未配置返回空串，走默认桩）。
func channelProviderCfg(ctx context.Context, sc *svc.ServiceContext, tid int64, channel string) string {
	ch, err := sc.Models.Channel.FindByCode(ctx, tid, channel)
	if err != nil {
		return ""
	}
	return ch.Provider
}

// parseEventPayload 解析 notification_request payload。
func parseEventPayload(raw json.RawMessage) (deliverInput, bool) {
	var p struct {
		UserId       int64             `json:"user_id"`
		TemplateCode string            `json:"template_code"`
		Params       map[string]string `json:"params"`
		BizType      string            `json:"biz_type"`
		BizId        string            `json:"biz_id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return deliverInput{}, false
	}
	return deliverInput{UserID: p.UserId, TemplateCode: p.TemplateCode, Params: p.Params, BizType: p.BizType, BizID: p.BizId}, true
}

// inQuietHours 免打扰时段判定（quiet_hours JSON {"start":"22:00","end":"08:00"}）。
func inQuietHours(quietJSON string, now time.Time) bool {
	if quietJSON == "" {
		return false
	}
	var q struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := json.Unmarshal([]byte(quietJSON), &q); err != nil || q.Start == "" || q.End == "" {
		return false
	}
	cur := now.Hour()*60 + now.Minute()
	start, sErr := parseHHMM(q.Start)
	end, eErr := parseHHMM(q.End)
	if sErr != nil || eErr != nil {
		return false
	}
	if start <= end {
		return cur >= start && cur < end
	}
	// 跨午夜窗口（22:00-08:00）
	return cur >= start || cur < end
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad hh:mm")
	}
	h, e1 := parseInt(parts[0])
	m, e2 := parseInt(parts[1])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("bad hh:mm")
	}
	return h*60 + m, nil
}

func parseInt(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("nan")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func statusOf(ok bool) string {
	if ok {
		return "SENT"
	}
	return "FAILED"
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
