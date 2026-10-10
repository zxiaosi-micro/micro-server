package logic

import (
	"context"
	"database/sql"

	"micro-server/services/station/internal/svc"
	"micro-server/services/station/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListStationByUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStationByUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationByUserLogic {
	return &ListStationByUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListStationByUser 用户驻站反查（S7 场站 Web data_scope 锁定本场站，FR-STN-006）。
func (l *ListStationByUserLogic) ListStationByUser(in *pb.ListStationByUserReq) (*pb.ListStationByUserResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 {
		return &pb.ListStationByUserResp{}, nil
	}
	staffs, err := l.svcCtx.Models.Staff.ListByUser(l.ctx, tid, in.UserId)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListStationByUserResp{}
	seen := map[int64]bool{}
	for _, s := range staffs {
		if seen[s.StationId] {
			continue
		}
		seen[s.StationId] = true
		st, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, s.StationId)
		if err != nil {
			continue
		}
		resp.List = append(resp.List, &pb.StationView{
			StationId: st.StationId, StationNo: st.StationNo, Name: st.Name,
			Type: st.Type, Status: st.Status, Province: st.Province, City: st.City,
			Address: st.Address, CapacityKwh: orFloat(st.CapacityKwh), PowerKw: orFloat(st.PowerKw),
			OrderNo: st.OrderNo.String, CreatedAt: st.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}

func orFloat(f sql.NullFloat64) float64 {
	if f.Valid {
		return f.Float64
	}
	return 0
}
