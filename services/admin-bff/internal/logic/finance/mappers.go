// finance pb ↔ types 映射辅助（E8）。

package finance

import (
	"strconv"

	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"
)

func paymentView(p *finpb.PaymentDetail) *types.PaymentView {
	return &types.PaymentView{
		PaymentNo: p.PaymentNo, OrderNo: p.OrderNo, Channel: p.Channel, Status: p.Status,
		Amount: p.Amount, PaidAmount: p.PaidAmount, ChannelTxnId: p.ChannelTxnId,
		PayerPartyId: strconv.FormatInt(p.PayerPartyId, 10),
		CreatedBy:    strconv.FormatInt(p.CreatedBy, 10),
		ApprovedBy:   strconv.FormatInt(p.ApprovedBy, 10),
		Remark:       p.Remark, PaidAt: p.PaidAt, CreatedAt: p.CreatedAt,
	}
}
