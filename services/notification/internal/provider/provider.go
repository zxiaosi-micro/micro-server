// 渠道 Provider 化（FR-NTF-001）：dev 用 log 桩；短信 Provider 上线前必切（04 §2 第 3 项）。
// 接口留位：新增渠道实现 Provider 接口并在 providerFor 注册即可。

package provider

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
)

// Provider 渠道投递器（outbound 结果由调用方落 outbound_log）。
type Provider interface {
	// Name Provider 标识（outbound_log.provider）。
	Name() string
	// Send 投递（target 手机号/邮箱/openid；content 已渲染）。
	Send(ctx context.Context, target, title, content string) error
}

// LogProvider log 桩：不触外网，内容落日志（dev 默认）。
type LogProvider struct{}

func (p *LogProvider) Name() string { return "log" }

func (p *LogProvider) Send(ctx context.Context, target, title, content string) error {
	logx.WithContext(ctx).Infof("[notification-stub] channel=sms target=%s title=%s content=%s", target, title, content)
	return nil
}

// For 渠道 → Provider 注册点（SmsProvider 留位：上线前实现并替换）。
func For(channel, providerCfg string) (Provider, error) {
	switch channel {
	case "WECHAT":
		// S7-02 订阅消息 Provider（FR-NTF-003）：dev MOCKWX 模拟，真实发送待模板 ID（04 §3 F2）。
		return &WechatProvider{cfg: parseWechatCfg(providerCfg)}, nil
	case "SMS", "EMAIL":
		// TODO(S11 上线准备): SMS 接云短信 Provider（阿里云/腾讯云），EMAIL 接 SMTP——
		// 均沿用本接口，出站结果落 outbound_log。
		return &LogProvider{}, nil
	case "INBOX":
		return nil, fmt.Errorf("provider: INBOX 为站内信落库，不走外发 Provider")
	default:
		return nil, fmt.Errorf("provider: 未知渠道 %s（provider_cfg=%s）", channel, providerCfg)
	}
}
