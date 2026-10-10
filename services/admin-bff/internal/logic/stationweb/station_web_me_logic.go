// stationweb BFF logic（S7-03：场站 Web 我的驻站信息，data_scope 锁定本场站）。

package stationweb

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	spb "micro-server/services/station/pb"
)

type StationWebMeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStationWebMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StationWebMeLogic {
	return &StationWebMeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// StationWebMe 登录身份（authz 注入 ctx）+ 驻站反查（FR-STN-006；多站取首个，前端锁定）。
func (l *StationWebMeLogic) StationWebMe(req *types.NoArgsReq) (*types.StationWebMeResp, error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	resp := &types.StationWebMeResp{
		UID:       strconv.FormatInt(uid, 10),
		Roles:     ctxkit.Roles(l.ctx),
		DataScope: ctxkit.DataScope(l.ctx),
	}
	if resp.Roles == nil {
		resp.Roles = []string{}
	}
	out, err := l.svcCtx.Station.ListStationByUser(l.ctx, &spb.ListStationByUserReq{UserId: uid})
	if err == nil && len(out.List) > 0 {
		resp.StationId = strconv.FormatInt(out.List[0].StationId, 10)
		resp.StationName = out.List[0].Name
		resp.StationNo = out.List[0].StationNo
	}
	return resp, nil
}
