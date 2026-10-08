// 事件消费入口 + 视图组装（contract）。

package logic

import (
	"time"

	"micro-server/services/contract/internal/model"

	ctpb "micro-server/services/contract/pb"
)

// ---- 视图组装 ----

func buildContractDetail(c *model.Contract, files []*model.ContractFile, targets []*model.ContractTarget) *ctpb.ContractDetail {
	d := &ctpb.ContractDetail{
		ContractId: c.ContractId,
		ContractNo: c.ContractNo,
		Type:       c.Type,
		Status:     c.Status,
		Name:       c.Name,
		TemplateId: c.TemplateId.Int64,
		Amount:     centsToAmount(floatToCents(c.Amount)),
		Remark:     nullStr(c.Remark),
		CreatedAt:  c.CreatedAt.UnixMilli(),
		CreatedBy:  c.CreatedBy.Int64,
	}
	if c.EffectiveAt.Valid {
		d.EffectiveAt = c.EffectiveAt.Time.UnixMilli()
	}
	if c.ArchivedAt.Valid {
		d.ArchivedAt = c.ArchivedAt.Time.UnixMilli()
	}
	for _, f := range files {
		d.Files = append(d.Files, &ctpb.ContractFile{
			FileRecId:     f.FileRecId,
			FileId:        f.FileId,
			FileName:      f.FileName,
			Version:       int32(f.Version),
			SignPartyName: nullStr(f.SignPartyName),
			SignPartyType: f.SignPartyType,
			Status:        f.Status,
			UploadedBy:    f.UploadedBy,
			UploaderName:  nullStr(f.UploaderName),
			UploadedAt:    f.CreatedAt.UnixMilli(),
			Remark:        nullStr(f.Remark),
		})
	}
	for _, t := range targets {
		d.Targets = append(d.Targets, &ctpb.ContractTarget{
			Type:     t.TargetType,
			TargetId: t.TargetPk,
			TargetNo: nullStr(t.TargetNo),
		})
	}
	return d
}

func buildWarrantyItem(w *model.Warranty) *ctpb.WarrantyItem {
	d := &ctpb.WarrantyItem{
		WarrantyId: w.WarrantyId,
		WarrantyNo: w.WarrantyNo,
		Level:      w.Level,
		TargetType: w.TargetType,
		TargetId:   w.TargetId,
		TargetKey:  w.TargetKey,
		Status:     w.Status,
		Months:     int32(w.Months),
		StartRule:  nullStr(w.StartRule),
		SourceType: w.SourceType,
		SourceNo:   nullStr(w.SourceNo),
		CreatedAt:  w.CreatedAt.UnixMilli(),
	}
	if w.StartAt.Valid {
		d.StartAt = w.StartAt.Time.UnixMilli()
	}
	if w.EndAt.Valid {
		d.EndAt = w.EndAt.Time.UnixMilli()
	}
	return d
}

func buildClaimDetail(c *model.Claim) *ctpb.ClaimDetail {
	d := &ctpb.ClaimDetail{
		ClaimId:     c.ClaimId,
		ClaimNo:     c.ClaimNo,
		WarrantyId:  c.WarrantyId,
		WarrantyNo:  c.WarrantyNo,
		Type:        c.Type,
		Description: nullStr(c.Description),
		Status:      c.Status,
		SettleType:  nullStr(c.SettleType),
		CreatedAt:   c.CreatedAt.UnixMilli(),
	}
	if c.Amount.Valid {
		d.Amount = centsToAmount(floatToCents(c.Amount.Float64))
	}
	return d
}

func buildSlaStrategyDetail(s *model.SlaStrategy) *ctpb.SlaStrategyDetail {
	return &ctpb.SlaStrategyDetail{
		StrategyId:      s.StrategyId,
		Code:            s.Code,
		Name:            s.Name,
		Level:           s.Level,
		ResponseMinutes: int32(s.ResponseMinutes),
		ResolveMinutes:  int32(s.ResolveMinutes),
		CreatedAt:       s.CreatedAt.UnixMilli(),
	}
}

var _ = time.Now
