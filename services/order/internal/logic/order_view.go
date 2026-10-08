// 订单视图组装（GetOrder/ListOrder 共用；对外 ID int64，E8 由 BFF 字符串化）。

package logic

import (
	"context"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"

	orderpb "micro-server/services/order/pb"
)

func buildOrderDetail(ctx context.Context, sc *svc.ServiceContext, tid int64, o *model.Order, withItems bool) (*orderpb.OrderDetail, error) {
	d := &orderpb.OrderDetail{
		OrderId:      o.OrderId,
		OrderNo:      o.OrderNo,
		Type:         o.Type,
		Status:       o.Status,
		BuyerPartyId: o.BuyerPartyId.Int64,
		TotalAmount:  centsToAmount(floatToCents(o.TotalAmount)),
		CancelReason: nullStr(o.CancelReason),
		Remark:       nullStr(o.Remark),
		CreatedAt:    o.CreatedAt.UnixMilli(),
		CreatedBy:    o.CreatedBy.Int64,
	}
	if o.PayExpireAt.Valid {
		d.PayExpireAt = o.PayExpireAt.Time.UnixMilli()
	}
	if o.PaidAt.Valid {
		d.PaidAt = o.PaidAt.Time.UnixMilli()
	}
	if withItems {
		items, err := sc.Models.OrderItem.FindByOrder(ctx, tid, o.OrderId)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			d.Items = append(d.Items, &orderpb.OrderItem{
				ItemId:      it.ItemId,
				SkuId:       it.SkuId,
				SkuName:     it.SkuName,
				Sn:          nullStr(it.Sn),
				WarehouseId: it.WarehouseId,
				Qty:         int32(it.Qty),
				UnitPrice:   centsToAmount(floatToCents(it.UnitPrice)),
				Amount:      centsToAmount(floatToCents(it.Amount)),
				OutQty:      int32(it.OutQty),
			})
		}
	}
	return d, nil
}
