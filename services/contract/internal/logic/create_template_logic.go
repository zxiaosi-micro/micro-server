package logic

import (
	"context"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type CreateTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTemplateLogic {
	return &CreateTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateTemplateLogic) CreateTemplate(in *pb.CreateTemplateReq) (*pb.CreateTemplateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Code == "" || in.Name == "" || in.ContractType == "" {
		return nil, errTemplateBodyEmpty.WithMsg("code/name/contract_type 必填")
	}
	if in.Body == "" {
		return nil, errTemplateBodyEmpty
	}
	tplId := l.svcCtx.Snowflake.MustNextID()
	tpl := &model.ContractTemplate{
		TemplateId: tplId, Code: in.Code, Name: in.Name,
		ContractType: in.ContractType, Body: in.Body, Status: "ACTIVE",
		TenantId: tid, CreatedBy: sqlInt64(ctxkit.UID(l.ctx)), UpdatedBy: sqlInt64(ctxkit.UID(l.ctx)),
	}
	if in.VariablesJson != "" {
		tpl.VariablesJson = sqlString(in.VariablesJson)
	}
	if err := l.svcCtx.Models.Template.InsertTx(l.ctx, tpl); err != nil {
		if isDupKey(err) {
			return nil, errTemplateCodeUsed
		}
		return nil, err
	}
	return &pb.CreateTemplateResp{TemplateId: tplId}, nil
}
