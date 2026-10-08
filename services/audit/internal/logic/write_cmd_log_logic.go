package logic

import (
	"context"

	"micro-server/services/audit/internal/model"
	"micro-server/services/audit/internal/svc"
	"micro-server/services/audit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type WriteCmdLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWriteCmdLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WriteCmdLogLogic {
	return &WriteCmdLogLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// WriteCmdLog 追加指令审计（控制指令独立流，S7 指令链路对齐 02 §9.7）。
func (l *WriteCmdLogLogic) WriteCmdLog(in *pb.WriteCmdLogReq) (*pb.WriteCmdLogResp, error) {
	if in.CmdId == "" || in.Sn == "" {
		return nil, errCmdRequired
	}
	if !validJSON(in.PayloadJson) {
		return nil, errcode.ErrBadRequest.WithMsg("payload_json 不合法")
	}
	tid, err := auditTenant(l.ctx, in.TenantId)
	if err != nil {
		return nil, err
	}

	cid := l.svcCtx.Snowflake.MustNextID()
	_, err = l.svcCtx.Models.CmdAudit.Insert(l.ctx, &model.CmdAudit{
		CmdAuditId:  cid,
		CmdId:       in.CmdId,
		Sn:          in.Sn,
		Uid:         in.Uid,
		Action:      in.Action,
		PayloadJson: toNullString(in.PayloadJson),
		Result:      defaultStr(in.Result, "SENT"),
		Error:       toNullString(in.Error),
		TraceId:     in.TraceId,
		TenantId:    tid,
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.WriteCmdLogResp{CmdAuditId: cid}, nil
}
