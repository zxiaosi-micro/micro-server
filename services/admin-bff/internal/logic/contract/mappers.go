// contract pb ↔ types 映射辅助（E8）。

package contract

import (
	"strconv"

	"micro-server/services/admin-bff/internal/types"
	ctpb "micro-server/services/contract/pb"
)

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func contractView(c *ctpb.ContractDetail) *types.ContractView {
	v := &types.ContractView{
		ContractId: strconv.FormatInt(c.ContractId, 10), ContractNo: c.ContractNo,
		Type: c.Type, Status: c.Status, Name: c.Name,
		TemplateId: strconv.FormatInt(c.TemplateId, 10), Amount: c.Amount,
		EffectiveAt: c.EffectiveAt, ArchivedAt: c.ArchivedAt,
		Remark: c.Remark, CreatedAt: c.CreatedAt,
		CreatedBy: strconv.FormatInt(c.CreatedBy, 10),
	}
	for _, f := range c.Files {
		v.Files = append(v.Files, types.ContractFileView{
			FileRecId: strconv.FormatInt(f.FileRecId, 10), FileId: f.FileId,
			FileName: f.FileName, Version: int(f.Version),
			SignPartyName: f.SignPartyName, SignPartyType: f.SignPartyType,
			Status: f.Status, UploadedBy: strconv.FormatInt(f.UploadedBy, 10),
			UploaderName: f.UploaderName, UploadedAt: f.UploadedAt, Remark: f.Remark,
		})
	}
	return v
}

func warrantyView(w *ctpb.WarrantyItem) *types.WarrantyView {
	return &types.WarrantyView{
		WarrantyNo: w.WarrantyNo, Level: w.Level, TargetType: w.TargetType,
		TargetId: strconv.FormatInt(w.TargetId, 10), TargetKey: w.TargetKey,
		Status: w.Status, StartAt: w.StartAt, EndAt: w.EndAt, Months: int(w.Months),
		StartRule: w.StartRule, SourceType: w.SourceType, SourceNo: w.SourceNo,
		CreatedAt: w.CreatedAt,
	}
}
