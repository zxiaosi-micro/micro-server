// 索赔（FR-CTR-008）与延保（FR-CTR-006）内核。

package logic

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

// CreateClaimInternal 索赔申请（须关联质保；质保外索赔转付费报价由 settle_type=PAID 承接）。
func CreateClaimInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	warrantyId int64, claimType, description string, uid int64) (int64, string, error) {

	if claimType != "QUALITY" && claimType != "TRANSPORT" && claimType != "INSTALL" {
		return 0, "", errClaimStatus.WithMsg("索赔类型不合法(QUALITY/TRANSPORT/INSTALL)")
	}
	w, err := sc.Models.Warranty.FindOneScoped(ctx, tid, warrantyId)
	if err != nil {
		if err == model.ErrNotFound {
			return 0, "", errWarrantyNotFound
		}
		return 0, "", err
	}

	claimId := sc.Snowflake.MustNextID()
	claimNo := "CLM" + fmt.Sprint(claimId)
	claim := &model.Claim{
		ClaimId: claimId, ClaimNo: claimNo,
		WarrantyId: w.WarrantyId, WarrantyNo: w.WarrantyNo,
		Type: claimType, Status: "APPLYING",
		TenantId: tid, CreatedBy: sqlInt64(uid), UpdatedBy: sqlInt64(uid),
	}
	if description != "" {
		claim.Description = sqlString(description)
	}
	if err := sc.Models.Claim.InsertTx(ctx, sc.Conn, claim); err != nil {
		if isDupKey(err) {
			return 0, "", errClaimStatus.WithMsg("索赔号冲突(重试)")
		}
		return 0, "", err
	}
	logx.WithContext(ctx).Infof("索赔已建 claim_no=%s warranty_no=%s type=%s by=%d", claimNo, w.WarrantyNo, claimType, ctxkit.UID(ctx))
	return claimId, claimNo, nil
}

// ApproveClaimInternal 索赔审批（APPLYING → APPROVED/REJECTED）。
func ApproveClaimInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	claimNo string, approve bool, remark string, uid int64) error {

	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		c, err := sc.Models.Claim.FindOneByNoForUpdateTx(ctx, session, tid, claimNo)
		if err != nil {
			return err
		}
		if c.Status != "APPLYING" {
			return errClaimStatus.WithMsg("当前状态: " + c.Status)
		}
		if approve {
			return sc.Models.Claim.CASStatusTx(ctx, session, tid, c.ClaimId, []string{"APPLYING"}, "APPROVED")
		}
		return sc.Models.Claim.MarkRejectedTx(ctx, session, tid, c.ClaimId, remark)
	})
	if err != nil {
		if err == model.ErrNotFound {
			return errClaimNotFound
		}
		return err
	}
	logx.WithContext(ctx).Infof("索赔审批完成 claim_no=%s approve=%v by=%d", claimNo, approve, ctxkit.UID(ctx))
	return nil
}

// SettleClaimInternal 索赔结算（APPROVED → SETTLED；REFUND/PAID 金额必填——结算联动 finance 为 L2 对接位）。
func SettleClaimInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	claimNo, settleType, amount string, uid int64) error {

	if settleType != "REPAIR" && settleType != "REPLACE" && settleType != "REFUND" && settleType != "PAID" {
		return errClaimStatus.WithMsg("结算方式不合法(REPAIR/REPLACE/REFUND/PAID)")
	}
	var amountF float64
	if settleType == "REFUND" || settleType == "PAID" {
		cents, err := parseCents(amount)
		if err != nil || cents <= 0 {
			return errAmountBad.WithMsg("REFUND/PAID 结算金额必填")
		}
		amountF = float64(cents) / 100
	}
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		c, err := sc.Models.Claim.FindOneByNoForUpdateTx(ctx, session, tid, claimNo)
		if err != nil {
			return err
		}
		if c.Status != "APPROVED" {
			return errClaimStatus.WithMsg("当前状态: " + c.Status)
		}
		return sc.Models.Claim.MarkSettledTx(ctx, session, tid, c.ClaimId, settleType, amountF, time.Now())
	})
	if err != nil {
		if err == model.ErrNotFound {
			return errClaimNotFound
		}
		return err
	}
	logx.WithContext(ctx).Infof("索赔已结算 claim_no=%s type=%s amount=%s by=%d", claimNo, settleType, amount, ctxkit.UID(ctx))
	return nil
}

// SellExtensionInternal 延保销售（衔接原质保：原质保 end_at + months，FR-CTR-006）。
func SellExtensionInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	baseWarrantyId int64, months int32, orderNo, amount string, uid int64) (int64, string, error) {

	if months <= 0 {
		return 0, "", errMonthsBad
	}
	amountCents, err := parseCents(amount)
	if err != nil {
		return 0, "", errAmountBad
	}
	w, err := sc.Models.Warranty.FindOneScoped(ctx, tid, baseWarrantyId)
	if err != nil {
		if err == model.ErrNotFound {
			return 0, "", errWarrantyNotFound
		}
		return 0, "", err
	}

	extId := sc.Snowflake.MustNextID()
	extNo := "EXT" + fmt.Sprint(extId)
	ext := &model.WarrantyExtension{
		ExtensionId: extId, ExtensionNo: extNo,
		BaseWarrantyId: w.WarrantyId, Months: int64(months),
		Amount: float64(amountCents) / 100, Status: "ACTIVE",
		TenantId: tid, CreatedBy: sqlInt64(uid), UpdatedBy: sqlInt64(uid),
	}
	if orderNo != "" {
		ext.OrderNo = sqlString(orderNo)
	}
	if err := sc.Models.Extension.InsertTx(ctx, sc.Conn, ext); err != nil {
		if isDupKey(err) {
			return 0, "", errExtensionStatus.WithMsg("延保号冲突(重试)")
		}
		return 0, "", err
	}
	logx.WithContext(ctx).Infof("延保已售 ext_no=%s base=%s months=%d by=%d", extNo, w.WarrantyNo, months, ctxkit.UID(ctx))
	return extId, extNo, nil
}

// TransferExtensionInternal 延保转移（换设备/场站，FR-CTR-006）。
func TransferExtensionInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	extNo, toTargetType string, toTargetId int64, toTargetKey string) error {

	ext, err := sc.Models.Extension.FindOneByNo(ctx, tid, extNo)
	if err != nil {
		if err == model.ErrNotFound {
			return errExtensionNotFound
		}
		return err
	}
	if ext.Status != "ACTIVE" {
		return errExtensionStatus.WithMsg("当前状态: " + ext.Status)
	}
	if toTargetType != "DEVICE" && toTargetType != "STATION" {
		return errTargetBad
	}
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return sc.Models.Extension.MarkTransferredTx(ctx, session, tid, ext.ExtensionId, toTargetType, toTargetId, toTargetKey)
	})
	if err != nil {
		if err == model.ErrStatusConflict {
			return errExtensionStatus
		}
		return err
	}
	logx.WithContext(ctx).Infof("延保已转移 ext_no=%s → %s:%s", extNo, toTargetType, toTargetKey)
	return nil
}

// RefundExtensionInternal 延保退款（状态置 REFUNDED；退款单联动 finance 为 L2 对接位——基线全额退）。
func RefundExtensionInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	extNo, reason string, uid int64) error {

	ext, err := sc.Models.Extension.FindOneByNo(ctx, tid, extNo)
	if err != nil {
		if err == model.ErrNotFound {
			return errExtensionNotFound
		}
		return err
	}
	if ext.Status != "ACTIVE" {
		return errExtensionStatus.WithMsg("当前状态: " + ext.Status)
	}
	refundNo := "RFD-EXT" + fmt.Sprint(ext.ExtensionId)
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.Extension.MarkRefundedTx(ctx, session, tid, ext.ExtensionId, refundNo); err != nil {
			return err
		}
		// 衔接的原质保同步作废（延保期内无缝衔接的反向：退款 → 原质保 REFUNDED）
		return sc.Models.Warranty.MarkRefundedTx(ctx, session, tid, ext.BaseWarrantyId)
	})
	if err != nil {
		if err == model.ErrStatusConflict {
			return errExtensionStatus
		}
		return err
	}
	logx.WithContext(ctx).Infof("延保已退款 ext_no=%s reason=%s by=%d", extNo, reason, ctxkit.UID(ctx))
	return nil
}
