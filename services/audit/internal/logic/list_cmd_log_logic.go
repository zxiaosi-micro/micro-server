package logic

import (
	"context"

	"micro-server/services/audit/internal/svc"
	"micro-server/services/audit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListCmdLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCmdLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCmdLogLogic {
	return &ListCmdLogLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListCmdLogLogic) ListCmdLog(in *pb.ListCmdLogReq) (*pb.ListCmdLogResp, error) {
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.CmdAudit.FindPage(l.ctx, in.Sn, in.CmdId, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListCmdLogResp{Total: total}
	for _, r := range list {
		resp.List = append(resp.List, &pb.CmdAuditItem{
			CmdAuditId:  r.CmdAuditId,
			CmdId:       r.CmdId,
			Sn:          r.Sn,
			Uid:         r.Uid,
			Action:      r.Action,
			PayloadJson: r.PayloadJson.String,
			Result:      r.Result,
			Error:       r.Error.String,
			TraceId:     r.TraceId,
			CreatedAt:   r.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
