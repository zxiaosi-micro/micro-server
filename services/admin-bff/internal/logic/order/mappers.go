// S5 pb ↔ types 映射辅助（E8：int64 ID → string）。

package order

import (
	"strconv"

	"micro-server/services/admin-bff/internal/types"
	opb "micro-server/services/order/pb"
)

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func orderView(o *opb.OrderDetail) *types.OrderView {
	v := &types.OrderView{
		OrderId: strconv.FormatInt(o.OrderId, 10), OrderNo: o.OrderNo,
		Type: o.Type, Status: o.Status,
		BuyerPartyId: strconv.FormatInt(o.BuyerPartyId, 10),
		TotalAmount:  o.TotalAmount, PayExpireAt: o.PayExpireAt, PaidAt: o.PaidAt,
		CancelReason: o.CancelReason, Remark: o.Remark,
		CreatedAt: o.CreatedAt, CreatedBy: strconv.FormatInt(o.CreatedBy, 10),
	}
	for _, it := range o.Items {
		v.Items = append(v.Items, types.OrderItemView{
			ItemId: strconv.FormatInt(it.ItemId, 10), SkuId: strconv.FormatInt(it.SkuId, 10),
			SkuName: it.SkuName, Sn: it.Sn, WarehouseId: strconv.FormatInt(it.WarehouseId, 10),
			Qty: int(it.Qty), UnitPrice: it.UnitPrice, Amount: it.Amount, OutQty: int(it.OutQty),
		})
	}
	return v
}
