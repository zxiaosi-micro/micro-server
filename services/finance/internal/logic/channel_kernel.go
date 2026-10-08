// 渠道内核（S5-03）：PayURL 取链 + 回调验签落账（微信 v3 / 支付宝 RSA2 / dev mock）。

package logic

import (
	"context"
	"encoding/json"
	"fmt"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// WechatPayURL 微信支付链接（未配置商户号 → dev mock URL）。
func WechatPayURL(ctx context.Context, sc *svc.ServiceContext, tid int64, paymentNo string) (string, error) {
	p, err := sc.Models.Payment.FindOneByNo(ctx, tid, paymentNo)
	if err != nil {
		if err == model.ErrNotFound {
			return "", errPaymentNotFound
		}
		return "", err
	}
	if p.Status != "PAYING" {
		return "", errPaymentStatus.WithMsg("当前状态: " + p.Status)
	}
	url, err := sc.Gateway.WechatNativeURL(ctx, p.PaymentNo, centsToAmount(floatToCents(p.Amount)), "订单支付 "+p.OrderNo)
	if err != nil {
		return "", err
	}
	logx.WithContext(ctx).Infof("微信支付链接已取 payment_no=%s mock=%v", paymentNo, !sc.Gateway.WechatNativeReady())
	return url, nil
}

// WechatCallback 微信 v3 回调：验签 → 解密资源 → 落账（ConfirmPayment）。
func WechatCallback(ctx context.Context, sc *svc.ServiceContext, tid int64, headersJSON, body string) (bool, error) {
	var h map[string]string
	if err := json.Unmarshal([]byte(headersJSON), &h); err != nil {
		return false, errChannelVerifyBad.WithMsg("回调头解析失败")
	}
	// 1. 验签（Wechatpay-Signature over timestamp\nnonce\nbody\n）
	if err := sc.Gateway.WechatVerifyCallback(h["Wechatpay-Timestamp"], h["Wechatpay-Nonce"], h["Wechatpay-Signature"], body); err != nil {
		logx.WithContext(ctx).Errorf("微信回调验签失败: %v", err)
		return false, errChannelVerifyBad
	}
	// 2. 解密资源（AES-256-GCM）
	var wrapped struct {
		Resource struct {
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
			Ciphertext     string `json:"ciphertext"`
		} `json:"resource"`
	}
	if err := json.Unmarshal([]byte(body), &wrapped); err != nil {
		return false, err
	}
	plain, err := sc.Gateway.WechatDecryptResource(wrapped.Resource.AssociatedData, wrapped.Resource.Nonce, wrapped.Resource.Ciphertext)
	if err != nil {
		logx.WithContext(ctx).Errorf("微信回调资源解密失败: %v", err)
		return false, errChannelVerifyBad.WithMsg("资源解密失败")
	}
	// 3. 落账
	var res struct {
		OutTradeNo string `json:"out_trade_no"`
		TransactionId string `json:"transaction_id"`
		Amount     struct {
			Total int64 `json:"total"` // 分
		} `json:"amount"`
	}
	if err := json.Unmarshal(plain, &res); err != nil {
		return false, err
	}
	if res.OutTradeNo == "" {
		return false, errChannelVerifyBad.WithMsg("回调缺 out_trade_no")
	}
	_, err = ConfirmPayment(ctx, sc, tid, res.OutTradeNo, res.TransactionId, centsToAmount(res.Amount.Total), "WECHAT_CALLBACK")
	if err != nil {
		return false, err
	}
	return true, nil
}

// AlipayPayURL 支付宝支付链接（未配置 → dev mock URL）。
func AlipayPayURL(ctx context.Context, sc *svc.ServiceContext, tid int64, paymentNo string) (string, error) {
	p, err := sc.Models.Payment.FindOneByNo(ctx, tid, paymentNo)
	if err != nil {
		if err == model.ErrNotFound {
			return "", errPaymentNotFound
		}
		return "", err
	}
	if p.Status != "PAYING" {
		return "", errPaymentStatus.WithMsg("当前状态: " + p.Status)
	}
	url, err := sc.Gateway.AlipayPayURL(ctx, p.PaymentNo, centsToAmount(floatToCents(p.Amount)), "订单支付 "+p.OrderNo)
	if err != nil {
		return "", err
	}
	return url, nil
}

// AlipayCallback 支付宝异步通知：RSA2 验签 → 落账。
func AlipayCallback(ctx context.Context, sc *svc.ServiceContext, tid int64, formJSON string) (bool, error) {
	var form map[string]string
	if err := json.Unmarshal([]byte(formJSON), &form); err != nil {
		return false, errChannelVerifyBad.WithMsg("通知表单解析失败")
	}
	if err := sc.Gateway.AlipayVerifyCallback(form); err != nil {
		logx.WithContext(ctx).Errorf("支付宝回调验签失败: %v", err)
		return false, errChannelVerifyBad
	}
	if status := form["trade_status"]; status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" {
		logx.WithContext(ctx).Infof("支付宝通知非成功态 status=%s（受理忽略）", status)
		return true, nil
	}
	paymentNo := form["out_trade_no"]
	if paymentNo == "" {
		return false, errChannelVerifyBad.WithMsg("通知缺 out_trade_no")
	}
	_, err := ConfirmPayment(ctx, sc, tid, paymentNo, form["trade_no"], form["total_amount"], "ALIPAY_CALLBACK")
	if err != nil {
		return false, err
	}
	return true, nil
}

var _ = fmt.Sprintf
