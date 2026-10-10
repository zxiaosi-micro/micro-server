// Logic 结构体定义（goctl 生成骨架的等价物——方法体实现分散在 kernel 文件；
// 重新生成 goctl 时 _logic.go 骨架会与本文件冲突，删除生成骨架保留本文件）。
package logic

import (
	"context"

	"micro-server/services/device/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportSNLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewImportSNLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportSNLogic {
	return &ImportSNLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceLogic {
	return &GetDeviceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDeviceLogic {
	return &ListDeviceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListByOrderNoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListByOrderNoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListByOrderNoLogic {
	return &ListByOrderNoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type TransitionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionLogic {
	return &TransitionLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ActivateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewActivateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActivateLogic {
	return &ActivateLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ProvisionCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProvisionCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvisionCredentialLogic {
	return &ProvisionCredentialLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetDeviceShadowLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDeviceShadowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceShadowLogic {
	return &GetDeviceShadowLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetDeviceTopologyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDeviceTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceTopologyLogic {
	return &GetDeviceTopologyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type SaveDeviceTopologyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveDeviceTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveDeviceTopologyLogic {
	return &SaveDeviceTopologyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type SendCommandLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendCommandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendCommandLogic {
	return &SendCommandLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListCmdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCmdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCmdLogic {
	return &ListCmdLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type SaveFirmwareLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveFirmwareLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveFirmwareLogic {
	return &SaveFirmwareLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListFirmwareLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFirmwareLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFirmwareLogic {
	return &ListFirmwareLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type CreateOtaTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOtaTaskLogic {
	return &CreateOtaTaskLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetOtaTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOtaTaskLogic {
	return &GetOtaTaskLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListOtaTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOtaTaskLogic {
	return &ListOtaTaskLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type RollbackOtaTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRollbackOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackOtaTaskLogic {
	return &RollbackOtaTaskLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetDeviceBySnLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDeviceBySnLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceBySnLogic {
	return &GetDeviceBySnLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}
