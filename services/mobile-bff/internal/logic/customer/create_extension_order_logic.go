// create_extension_order_logic.go · 延保购买（统一下单链路：EXT_WARRANTY SKU 走
// order.CreateOrder + PayOrder(WECHAT)，dev 微信支付走模拟网关；仓库由 BFF 代选首个启用仓，
// C 端不感知仓库概念。base_warranty_id 进订单 remark 留痕，合同域自动衔接为演进项）。

package customer

import (
	"context"
	"fmt"
	"time"

	ctpb "micro-server/services/contract/pb"
	dpb "micro-server/services/device/pb"
	invpb "micro-server/services/inventory/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

// errWarrantyNotMine 原质保不存在或不属于当前账号（平台段 110405）。
var errWarrantyNotMine = errcode.New(errcode.SegPlatform, 405, "原质保不存在或不属于当前账号")

type CreateExtensionOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateExtensionOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateExtensionOrderLogic {
	return &CreateExtensionOrderLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateExtensionOrderLogic) CreateExtensionOrder(req *types.CreateExtensionOrderReq) (resp *types.CreateExtensionOrderResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	// 原质保归属校验（防横向越权衔接他人质保）
	if err := l.assertOwnWarranty(partyId, req.BaseWarrantyId); err != nil {
		return nil, err
	}

	// 出库仓代选（首个启用仓；延保 SKU 也走统一下单/库存锁链路）
	wh, err := l.svcCtx.Inventory.ListWarehouse(l.ctx, &invpb.ListWarehouseReq{Page: 1, Size: 1})
	if err != nil || len(wh.List) == 0 {
		if err == nil {
			err = fmt.Errorf("无可用仓库")
		}
		return nil, err
	}

	created, err := l.svcCtx.Order.CreateOrder(l.ctx, &opb.CreateOrderReq{
		Type:         "DEVICE",
		BuyerPartyId: partyId,
		Remark:       fmt.Sprintf("延保购买 base_warranty_id=%s", req.BaseWarrantyId),
		Items: []*opb.OrderItemInput{{
			SkuId:       parseI64(req.SkuId),
			WarehouseId: wh.List[0].WarehouseId,
			Qty:         int32(req.Qty),
		}},
	})
	if err != nil {
		return nil, err
	}

	// Saga 步骤 2 锁库存由 cron 30s 扫描推进（order.go registerCron），PayOrder 要求 LOCKED——
	// 有界等待（40s 覆盖一个扫描周期）；演进位：拆两段接口由客户端轮询支付状态（contract 不变式：一次性返回 pay_params 保留）
	paid, err := l.payWhenLocked(created.OrderNo)
	if err != nil {
		return nil, err
	}
	return &types.CreateExtensionOrderResp{
		OrderNo:     created.OrderNo,
		TotalAmount: created.TotalAmount,
		PayExpireAt: created.PayExpireAt,
		PaymentNo:   paid.PaymentNo,
		PayParams:   paid.PayParams,
	}, nil
}

// payWhenLocked 等订单 LOCKED 后发起微信支付（见 CreateExtensionOrder 注释）。
func (l *CreateExtensionOrderLogic) payWhenLocked(orderNo string) (*opb.PayOrderResp, error) {
	deadline := time.Now().Add(40 * time.Second)
	for {
		o, err := l.svcCtx.Order.GetOrder(l.ctx, &opb.GetOrderReq{OrderNo: orderNo})
		if err != nil {
			return nil, err
		}
		switch o.GetOrder().GetStatus() {
		case "LOCKED":
			return l.svcCtx.Order.PayOrder(l.ctx, &opb.PayOrderReq{OrderNo: orderNo, Channel: "WECHAT"})
		case "PAYING":
			// 已在支付中（重放）：直接查支付参数
			return l.svcCtx.Order.PayOrder(l.ctx, &opb.PayOrderReq{OrderNo: orderNo, Channel: "WECHAT"})
		case "CREATED":
			if time.Now().After(deadline) {
				return nil, errcode.Internal.WithMsg("库存锁定中，请稍后重试")
			}
			time.Sleep(2 * time.Second)
		default:
			return nil, errcode.Internal.WithMsg("订单状态不可支付: " + o.GetOrder().GetStatus())
		}
	}
}

// assertOwnWarranty 校验 base_warranty_id 属于本人设备的质保（O(设备数) 扇出，C 端规模有界）。
func (l *CreateExtensionOrderLogic) assertOwnWarranty(partyId int64, baseWarrantyId string) error {
	devices, err := l.svcCtx.Device.ListDevice(l.ctx, &dpb.ListDeviceReq{PartyId: partyId, Page: 1, Size: 100})
	if err != nil {
		return err
	}
	for _, d := range devices.List {
		w, err := l.svcCtx.Contract.GetWarrantyByTarget(l.ctx, &ctpb.GetWarrantyByTargetReq{
			TargetType: "DEVICE", TargetId: d.DeviceId,
		})
		if err != nil {
			continue
		}
		for _, wi := range w.List {
			if i64s(wi.WarrantyId) == baseWarrantyId {
				return nil
			}
		}
	}
	return errWarrantyNotMine
}
