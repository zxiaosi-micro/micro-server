// warranties_logic.go · 我的质保（本人设备逐台 GetWarrantyByTarget 归并；
// party_id 隔离由设备页规模保证——C 端设备数小，页内扇出有界）。

package customer

import (
	"context"

	ctpb "micro-server/services/contract/pb"
	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WarrantiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWarrantiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WarrantiesLogic {
	return &WarrantiesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *WarrantiesLogic) Warranties(req *types.WarrantyListReq) (resp *types.WarrantyListResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Device.ListDevice(l.ctx, &dpb.ListDeviceReq{
		PartyId: partyId,
		Page:    int32(req.Page),
		Size:    int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.WarrantyListResp{List: []types.WarrantyView{}}
	for _, d := range out.List {
		w, err := l.svcCtx.Contract.GetWarrantyByTarget(l.ctx, &ctpb.GetWarrantyByTargetReq{
			TargetType: "DEVICE", TargetId: d.DeviceId,
		})
		if err != nil {
			continue // 单设备质保查询失败不阻塞整页（下一次轮询/刷新自然收敛）
		}
		for _, wi := range w.List {
			resp.List = append(resp.List, pbWarranty(wi))
		}
	}
	resp.Total = int64(len(resp.List))
	return resp, nil
}
