// WechatProvider 微信订阅消息渠道（FR-NTF-003，S7-02 L2）。
//
// dev 模拟口径（04 §3 F2）：MOCKWX=1 或 provider_cfg 缺 app_id → 结构化日志桩，
// outbound_log 照记（链路可验，不触外网）。真实发送需三前置：
//  1. 微信后台申请订阅消息模板 ID（用户侧动作，04 §3 F2）；
//  2. 用户 openid 注册（S9 客户端小程序 wx.login 后回传，identity 落 user_ext）；
//  3. 小程序端 askSubscribeMessage 授权（S9，S7-02 验收用 dev mock 模拟授权后触达）。
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// wechatCfg provider_cfg JSON（notification_channel.provider 列）。
type wechatCfg struct {
	AppID      string `json:"app_id"`      // 小程序 AppID
	Secret     string `json:"secret"`      // 小程序 AppSecret（access_token 换取用）
	TemplateID string `json:"template_id"` // 订阅消息模板 ID（告警/工单/账单各一条，多模板经模板表映射）
	Mock       bool   `json:"mock"`        // 强制 mock（不触外网，outbound_log 照记）
}

// WechatProvider 订阅消息渠道实现（dev 无模板时走 mock 桩，outbound_log 照记）。
type WechatProvider struct {
	cfg wechatCfg // 渠道配置（app_id/template_id/mock）
}

func (p *WechatProvider) Name() string { return "wechat" }

// Send 订阅消息投递（target 现为 user:<uid> 占位；openid 映射 S9 接入后替换）。
func (p *WechatProvider) Send(ctx context.Context, target, title, content string) error {
	if p.mock() {
		logx.WithContext(ctx).Infof("[notification-mockwx] channel=wechat target=%s title=%s content=%s（MOCKWX 模拟：订阅消息已触达）",
			target, title, content)
		return nil
	}
	// 真实链路（模板 ID 就绪后启用）：access_token 缓存 → POST cgi-bin/message/subscribe/send
	// data 字段映射：thing1=title / thing2=content（模板字段以申请结果为准）。
	return fmt.Errorf("wechat: 订阅消息模板 ID 未就绪（04 §3 F2 前置），MOCKWX=1 走 dev 模拟")
}

// mock dev 模拟判定：MOCKWX=1 环境变量 或 配置缺 app_id。
func (p *WechatProvider) mock() bool {
	if strings.EqualFold(os.Getenv("MOCKWX"), "1") || p.cfg.AppID == "" {
		return true
	}
	return p.cfg.Mock
}

// parseWechatCfg provider_cfg 解析（空/坏 JSON = 全默认 → mock）。
func parseWechatCfg(cfg string) wechatCfg {
	var c wechatCfg
	if cfg != "" {
		_ = json.Unmarshal([]byte(cfg), &c)
	}
	return c
}
