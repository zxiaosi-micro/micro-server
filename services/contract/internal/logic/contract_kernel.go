// 合同内核（S5-04，FR-CTR-001~004）：随单建立 / 归档件五要素 / 状态机。
//
// 归档五要素（FR-CTR-003 验收口径）：
//   1. 上传人（uploaded_by 审计）   2. 版本（version 重签递增，旧版 REPLACED）
//   3. 关联（contract_target 订单/客户/场站可追溯）  4. 状态（ACTIVE/REPLACED）
//   5. 受控下载（file_id 存合同桶 micro-contract 版本控制；下载令牌走 file 服务，BFF step-up 面）
//
// 平台不解析文件内容（基线）；打印稿辅助（L2）走 RenderTemplate → Go 原生 PDF（ADR-18）。

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

// CreateFromOrderInternal Saga 步骤5：随单建合同（幂等：同单已有合同直接返回）+ 质保计划。
func CreateFromOrderInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	orderNo, orderType string, buyerPartyId int64, buyerName, amount, itemsJSON string, templateId int64, uid int64) (int64, string, int, error) {

	// 幂等：同单合同已存在（Saga 重试/事件重放）
	if existed, err := sc.Models.Contract.FindOneByOrderNo(ctx, tid, orderNo); err == nil {
		return existed.ContractId, existed.ContractNo, 0, nil
	} else if err != model.ErrNotFound {
		return 0, "", 0, err
	}

	contractType := "SALES"
	name := "销售合同 " + orderNo
	if orderType == "STATION" {
		contractType = "STATION"
		name = "场站合同 " + orderNo
	}
	amountCents, err := parseCents(amount)
	if err != nil {
		return 0, "", 0, errAmountBad
	}

	contractId := sc.Snowflake.MustNextID()
	contractNo := "CTR" + fmt.Sprint(contractId)
	warrantyCount := 0

	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		c := &model.Contract{
			ContractId: contractId, ContractNo: contractNo,
			Type: contractType, Status: "DRAFT", Name: name,
			TemplateId: sqlInt64(templateId), Amount: float64(amountCents) / 100,
			OrderNo: sqlString(orderNo),
			TenantId: tid, CreatedBy: sqlInt64(uid), UpdatedBy: sqlInt64(uid),
		}
		if buyerPartyId > 0 {
			c.BuyerPartyId = sqlInt64(buyerPartyId)
		}
		if buyerName != "" {
			c.Remark = sqlString("买方: " + buyerName)
		}
		if err := sc.Models.Contract.InsertTx(ctx, session, c); err != nil {
			return err
		}
		// 关联（归档五要素之「关联」）
		target := &model.ContractTarget{
			TargetRecId: sc.Snowflake.MustNextID(), ContractId: contractId,
			TargetType: "ORDER", TargetPk: 0, TargetNo: sqlString(orderNo),
			TenantId: tid, CreatedBy: sqlInt64(uid), UpdatedBy: sqlInt64(uid),
		}
		if buyerPartyId > 0 {
			pt := &model.ContractTarget{
				TargetRecId: sc.Snowflake.MustNextID(), ContractId: contractId,
				TargetType: "PARTY", TargetPk: buyerPartyId, TargetNo: sqlString(buyerName),
				TenantId: tid, CreatedBy: sqlInt64(uid), UpdatedBy: sqlInt64(uid),
			}
			if err := sc.Models.Target.InsertTx(ctx, session, pt); err != nil {
				return err
			}
		}
		if err := sc.Models.Target.InsertTx(ctx, session, target); err != nil {
			return err
		}
		// 质保计划（PENDING 待起算）
		var err error
		warrantyCount, err = CreateWarrantiesFromOrder(ctx, sc, tid, session, orderNo, orderType, itemsJSON)
		return err
	})
	if err != nil {
		if isDupKey(err) {
			// 并发建单：重读幂等返回
			if existed, ferr := sc.Models.Contract.FindOneByOrderNo(ctx, tid, orderNo); ferr == nil {
				return existed.ContractId, existed.ContractNo, 0, nil
			}
			return 0, "", 0, errContractNoUsed
		}
		return 0, "", 0, err
	}
	logx.WithContext(ctx).Infof("合同已建 contract_no=%s order_no=%s type=%s warranties=%d by=%d",
		contractNo, orderNo, contractType, warrantyCount, ctxkit.UID(ctx))
	return contractId, contractNo, warrantyCount, nil
}

// UploadContractFileInternal 归档件上传登记（版本递增；旧版置 REPLACED）。
func UploadContractFileInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	contractNo, fileId, fileName, signPartyName, signPartyType, remark string, uid int64, uploaderName string) (int64, error) {

	if fileId == "" {
		return 0, errFileIdRequired
	}
	if signPartyType == "" {
		signPartyType = "BUYER"
	}
	c, err := sc.Models.Contract.FindOneByNo(ctx, tid, contractNo)
	if err != nil {
		if err == model.ErrNotFound {
			return 0, errContractNotFound
		}
		return 0, err
	}
	if c.Status == "ARCHIVED" {
		return 0, errContractStatus.WithMsg("已归档合同不可再上传归档件")
	}

	var version int64
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		max, err := sc.Models.File.MaxVersion(ctx, tid, c.ContractId)
		if err != nil {
			return err
		}
		version = max + 1
		f := &model.ContractFile{
			FileRecId: sc.Snowflake.MustNextID(), ContractId: c.ContractId,
			FileId: fileId, FileName: fileName,
			Version: version, SignPartyType: signPartyType,
			Status: "ACTIVE", UploadedBy: uid, UploaderName: sqlString(uploaderName),
			TenantId: tid, CreatedBy: sqlInt64(uid), UpdatedBy: sqlInt64(uid),
		}
		if signPartyName != "" {
			f.SignPartyName = sqlString(signPartyName)
		}
		if remark != "" {
			f.Remark = sqlString(remark)
		}
		if err := sc.Models.File.InsertTx(ctx, session, f); err != nil {
			return err
		}
		// 重签：旧版本置 REPLACED（版本链完整保留）
		return sc.Models.File.ReplaceOldTx(ctx, session, tid, c.ContractId, version)
	})
	if err != nil {
		return 0, err
	}
	logx.WithContext(ctx).Infof("归档件已登记 contract_no=%s version=%d file_id=%s by=%d（step-up 由 BFF 面）",
		contractNo, version, fileId, ctxkit.UID(ctx))
	return version, nil
}

// ArchiveContractInternal 生效/归档（EFFECTIVE 须 ≥1 份 ACTIVE 归档件）。
func ArchiveContractInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	contractNo, action, remark string, uid int64) error {

	c, err := sc.Models.Contract.FindOneByNo(ctx, tid, contractNo)
	if err != nil {
		if err == model.ErrNotFound {
			return errContractNotFound
		}
		return err
	}
	switch action {
	case "EFFECTIVE":
		if c.Status != "DRAFT" {
			return errContractStatus.WithMsg("仅草稿可生效, 当前: " + c.Status)
		}
		n, err := sc.Models.File.CountActive(ctx, tid, c.ContractId)
		if err != nil {
			return err
		}
		if n == 0 {
			return errFileRequired // 归档五要素之「状态」：生效前须有已签归档件
		}
		err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			return sc.Models.Contract.MarkEffectiveTx(ctx, session, tid, c.ContractId, time.Now())
		})
		if err != nil {
			if err == model.ErrStatusConflict {
				return errContractStatus
			}
			return err
		}
		logx.WithContext(ctx).Infof("合同已生效 contract_no=%s by=%d remark=%s", contractNo, ctxkit.UID(ctx), remark)
	case "ARCHIVE":
		if c.Status != "ACTIVE" {
			return errContractStatus.WithMsg("仅生效中合同可归档, 当前: " + c.Status)
		}
		err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			return sc.Models.Contract.MarkArchivedTx(ctx, session, tid, c.ContractId, time.Now())
		})
		if err != nil {
			if err == model.ErrStatusConflict {
				return errContractStatus
			}
			return err
		}
		logx.WithContext(ctx).Infof("合同已归档 contract_no=%s by=%d", contractNo, ctxkit.UID(ctx))
	default:
		return errContractStatus.WithMsg("action 不合法(EFFECTIVE/ARCHIVE)")
	}
	return nil
}
