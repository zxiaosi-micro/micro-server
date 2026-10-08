# gen_bff_logic.py · 生成 admin-bff S4 业务域 logic 转发实现（一次性工具）
import io, os

ROOT = r"D:\Personal code\micro-new\micro-server\services\admin-bff\internal\logic"

HDR = '''// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package {pkg}

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	{alias} "micro-server/services/{svc}/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type {name}Logic struct {{
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}}

// {comment}
func New{name}Logic(ctx context.Context, svcCtx *svc.ServiceContext) *{name}Logic {{
	return &{name}Logic{{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}}
}}

'''

def w(group, name, svc_name, comment, body, extra_types=""):
    pkg = group
    alias = svc_name + "pb"
    content = HDR.format(pkg=pkg, name=name, comment=comment, svc=svc_name, alias=alias) + body + "\n"
    path = os.path.join(ROOT, group, snake(name) + "_logic.go")
    io.open(path, "w", encoding="utf-8", newline="\n").write(content)

def snake(n):
    out = []
    for i, c in enumerate(n):
        if c.isupper() and i > 0 and (not n[i-1].isupper() or (i+1 < len(n) and n[i+1].islower())):
            out.append("_")
        out.append(c.lower())
    s = "".join(out)
    return s.replace("s_k_u", "s_k_u").replace("u_nread", "unread")

# ---------- party ----------
w("party", "CreateParty", "party", "新建参与方(perm: party:party:create)", '''func (l *CreatePartyLogic) CreateParty(req *types.PartyCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.CreateParty(l.ctx, &partyPb.CreatePartyReq{
		Name: req.Name, Type: req.Type, CreditCode: req.CreditCode,
		Region: req.Region, Address: req.Address, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "ListParties", "party", "参与方列表(perm: party:party:list)", '''func (l *ListPartiesLogic) ListParties(req *types.PartyListReq) (resp *types.PartyListResp, err error) {
	r, err := l.svcCtx.Party.ListParty(l.ctx, &partyPb.ListPartyReq{
		Keyword: req.Keyword, Type: req.Type, Status: int32(req.Status),
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.PartyListResp{Total: r.Total}
	for _, p := range r.List {
		resp.List = append(resp.List, types.PartyItem{
			PartyID: strconv.FormatInt(p.PartyId, 10), Name: p.Name,
			Type: parseStringSlice(p.Type), Status: int(p.Status),
			CreditCode: p.CreditCode, Region: p.Region, Address: p.Address, Remark: p.Remark,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		})
	}
	return resp, nil
}''')

w("party", "GetParty", "party", "参与方详情(perm: party:party:list)", '''func (l *GetPartyLogic) GetParty(req *types.IDPath) (resp *types.PartyDetailResp, err error) {
	pid := parseID(req.Id)
	r, err := l.svcCtx.Party.GetParty(l.ctx, &partyPb.GetPartyReq{PartyId: pid})
	if err != nil {
		return nil, err
	}
	p := r.Party
	return &types.PartyDetailResp{Party: types.PartyItem{
		PartyID: strconv.FormatInt(p.PartyId, 10), Name: p.Name,
		Type: parseStringSlice(p.Type), Status: int(p.Status),
		CreditCode: p.CreditCode, Region: p.Region, Address: p.Address, Remark: p.Remark,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}}, nil
}''')

w("party", "UpdateParty", "party", "编辑参与方(perm: party:party:update)", '''func (l *UpdatePartyLogic) UpdateParty(req *types.PartyUpdateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.UpdateParty(l.ctx, &partyPb.UpdatePartyReq{
		PartyId: parseID(req.Id), Name: req.Name, Type: req.Type, Status: int32(req.Status),
		CreditCode: req.CreditCode, Region: req.Region, Address: req.Address, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "ListContacts", "party", "联系人列表(perm: party:party:list)", '''func (l *ListContactsLogic) ListContacts(req *types.IDPath) (resp *types.ContactListResp, err error) {
	r, err := l.svcCtx.Party.ListContact(l.ctx, &partyPb.ListContactReq{PartyId: parseID(req.Id)})
	if err != nil {
		return nil, err
	}
	resp = &types.ContactListResp{}
	for _, c := range r.List {
		resp.List = append(resp.List, types.ContactItem{
			ContactID: strconv.FormatInt(c.ContactId, 10), PartyID: strconv.FormatInt(c.PartyId, 10),
			Name: c.Name, Mobile: c.Mobile, Position: c.Position,
			IsDefault: c.IsDefault, NotifyPref: c.NotifyPref, CreatedAt: c.CreatedAt,
		})
	}
	return resp, nil
}''')

w("party", "AddContact", "party", "新增联系人(perm: party:contact:create)", '''func (l *AddContactLogic) AddContact(req *types.ContactCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.AddContact(l.ctx, &partyPb.AddContactReq{
		PartyId: parseID(req.Id), Name: req.Name, Mobile: req.Mobile,
		Position: req.Position, IsDefault: req.IsDefault, NotifyPref: req.NotifyPref,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "ListCrmRecords", "party", "跟进记录(perm: party:party:list)", '''func (l *ListCrmRecordsLogic) ListCrmRecords(req *types.IDPath) (resp *types.CrmRecordListResp, err error) {
	r, err := l.svcCtx.Party.ListCrmRecord(l.ctx, &partyPb.ListCrmRecordReq{PartyId: parseID(req.Id)})
	if err != nil {
		return nil, err
	}
	resp = &types.CrmRecordListResp{}
	for _, c := range r.List {
		resp.List = append(resp.List, types.CrmRecordItem{
			RecordID: strconv.FormatInt(c.RecordId, 10), PartyID: strconv.FormatInt(c.PartyId, 10),
			Content: c.Content, NextFollowAt: c.NextFollowAt,
			CreatedBy: strconv.FormatInt(c.CreatedBy, 10), CreatedAt: c.CreatedAt,
		})
	}
	return resp, nil
}''')

w("party", "AddCrmRecord", "party", "追加跟进(perm: party:crm:create)", '''func (l *AddCrmRecordLogic) AddCrmRecord(req *types.CrmRecordCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.AddCrmRecord(l.ctx, &partyPb.AddCrmRecordReq{
		PartyId: parseID(req.Id), Content: req.Content, NextFollowAt: req.NextFollowAt,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "ListStaff", "party", "员工列表(perm: party:party:list)", '''func (l *ListStaffLogic) ListStaff(req *types.IDPath) (resp *types.StaffListResp, err error) {
	r, err := l.svcCtx.Party.ListStaff(l.ctx, &partyPb.ListStaffReq{PartyId: parseID(req.Id), Page: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	resp = &types.StaffListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.StaffItem{
			StaffID: strconv.FormatInt(s.StaffId, 10), PartyID: strconv.FormatInt(s.PartyId, 10),
			UserID: strconv.FormatInt(s.UserId, 10), Name: s.Name, StaffType: s.StaffType,
			SkillTags: parseStringSlice(s.SkillTags), WorkRegion: s.WorkRegion,
			Status: int(s.Status), CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}''')

w("party", "CreateStaff", "party", "新建员工(perm: party:staff:create)", '''func (l *CreateStaffLogic) CreateStaff(req *types.StaffCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.CreateStaff(l.ctx, &partyPb.CreateStaffReq{
		PartyId: parseID(req.PartyId), UserId: parseID(req.UserID), Name: req.Name,
		StaffType: req.StaffType, SkillTags: req.SkillTags, WorkRegion: req.WorkRegion,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "GetDealerExt", "party", "经销商扩展(perm: party:party:list)", '''func (l *GetDealerExtLogic) GetDealerExt(req *types.IDPath) (resp *types.DealerExtResp, err error) {
	r, err := l.svcCtx.Party.GetDealerExt(l.ctx, &partyPb.GetDealerExtReq{PartyId: parseID(req.Id)})
	if err != nil {
		return nil, err
	}
	d := r.DealerExt
	return &types.DealerExtResp{DealerExt: types.DealerExtItem{
		PartyID: strconv.FormatInt(d.PartyId, 10), DealerLevel: d.DealerLevel,
		AuthorizedRegion: d.AuthorizedRegion, RebateRule: d.RebateRule, UpdatedAt: d.UpdatedAt,
	}}, nil
}''')

w("party", "UpsertDealerExt", "party", "经销商扩展 Upsert(perm: party:dealer:update)", '''func (l *UpsertDealerExtLogic) UpsertDealerExt(req *types.DealerExtUpsertReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.UpsertDealerExt(l.ctx, &partyPb.UpsertDealerExtReq{
		PartyId: parseID(req.Id), DealerLevel: req.DealerLevel,
		AuthorizedRegion: req.AuthorizedRegion, RebateRule: req.RebateRule,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "ListOpportunities", "party", "商机列表(perm: party:opportunity:list)", '''func (l *ListOpportunitiesLogic) ListOpportunities(req *types.OpportunityListReq) (resp *types.OpportunityListResp, err error) {
	r, err := l.svcCtx.Party.ListOpportunity(l.ctx, &partyPb.ListOpportunityReq{
		PartyId: parseID(req.PartyID), Stage: req.Stage, Keyword: req.Keyword,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.OpportunityListResp{Total: r.Total}
	for _, o := range r.List {
		resp.List = append(resp.List, types.OpportunityItem{
			OpportunityID: strconv.FormatInt(o.OpportunityId, 10), PartyID: strconv.FormatInt(o.PartyId, 10),
			Title: o.Title, Stage: o.Stage, Amount: o.Amount,
			ExpectedClose: o.ExpectedCloseDate, Remark: o.Remark,
			CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
		})
	}
	return resp, nil
}''')

w("party", "CreateOpportunity", "party", "新建商机(perm: party:opportunity:create)", '''func (l *CreateOpportunityLogic) CreateOpportunity(req *types.OpportunityCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.CreateOpportunity(l.ctx, &partyPb.CreateOpportunityReq{
		PartyId: parseID(req.PartyID), Title: req.Title, Amount: req.Amount,
		ExpectedCloseDate: req.ExpectedClose, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("party", "UpdateOpportunityStage", "party", "商机阶段推进(perm: party:opportunity:update)", '''func (l *UpdateOpportunityStageLogic) UpdateOpportunityStage(req *types.OpportunityStageReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.UpdateOpportunityStage(l.ctx, &partyPb.UpdateOpportunityStageReq{
		OpportunityId: parseID(req.Id), Stage: req.Stage,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

# ---------- catalog ----------
w("catalog", "ListProducts", "catalog", "商品列表(perm: catalog:product:list)", '''func (l *ListProductsLogic) ListProducts(req *types.ProductListReq) (resp *types.ProductListResp, err error) {
	r, err := l.svcCtx.Catalog.ListProduct(l.ctx, &catalogPb.ListProductReq{
		Keyword: req.Keyword, Status: int32(req.Status), Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.ProductListResp{Total: r.Total}
	for _, p := range r.List {
		resp.List = append(resp.List, types.ProductItem{
			ProductID: strconv.FormatInt(p.ProductId, 10), Name: p.Name, Category: p.Category,
			Status: int(p.Status), Remark: p.Remark, CreatedAt: p.CreatedAt,
		})
	}
	return resp, nil
}''')

w("catalog", "CreateProduct", "catalog", "新建商品(perm: catalog:product:create)", '''func (l *CreateProductLogic) CreateProduct(req *types.ProductCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.CreateProduct(l.ctx, &catalogPb.CreateProductReq{Name: req.Name, Category: req.Category, Remark: req.Remark})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("catalog", "ListSkus", "catalog", "SKU 列表(perm: catalog:sku:list)", '''func (l *ListSkusLogic) ListSkus(req *types.SkuListReq) (resp *types.SkuListResp, err error) {
	r, err := l.svcCtx.Catalog.ListSKU(l.ctx, &catalogPb.ListSKUReq{
		ProductId: parseID(req.ProductID), Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.SkuListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.SkuItem{
			SkuID: strconv.FormatInt(s.SkuId, 10), ProductID: strconv.FormatInt(s.ProductId, 10),
			Code: s.Code, Name: s.Name, Type: s.Type, Spec: s.Spec,
			Status: int(s.Status), CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}''')

w("catalog", "CreateSku", "catalog", "新建 SKU(perm: catalog:sku:create)", '''func (l *CreateSkuLogic) CreateSku(req *types.SkuCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.CreateSKU(l.ctx, &catalogPb.CreateSKUReq{
		ProductId: parseID(req.ProductID), Code: req.Code, Name: req.Name, Type: req.Type, Spec: req.Spec,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("catalog", "ListPrices", "catalog", "SKU 价格列表(版本化,perm: catalog:price:list)", '''func (l *ListPricesLogic) ListPrices(req *types.PriceListReq) (resp *types.PriceListResp, err error) {
	r, err := l.svcCtx.Catalog.ListPrice(l.ctx, &catalogPb.ListPriceReq{SkuId: parseID(req.SkuID), LatestOnly: req.LatestOnly})
	if err != nil {
		return nil, err
	}
	resp = &types.PriceListResp{Total: r.Total}
	for _, p := range r.List {
		resp.List = append(resp.List, types.PriceItem{
			PriceID: strconv.FormatInt(p.PriceId, 10), SkuID: strconv.FormatInt(p.SkuId, 10),
			PriceType: p.PriceType, TierQty: int(p.TierQty), Amount: p.Amount,
			Version: int(p.Version), CreatedAt: p.CreatedAt,
		})
	}
	return resp, nil
}''')

w("catalog", "SetPrice", "catalog", "设置价格(新版本落库,perm: catalog:price:set)", '''func (l *SetPriceLogic) SetPrice(req *types.PriceSetReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.SetPrice(l.ctx, &catalogPb.SetPriceReq{
		SkuId: parseID(req.Id), PriceType: req.PriceType, TierQty: int32(req.TierQty), Amount: req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("catalog", "UpsertWarranty", "catalog", "质保策略 Upsert(perm: catalog:warranty:update)", '''func (l *UpsertWarrantyLogic) UpsertWarranty(req *types.WarrantyPolicyUpsertReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.UpsertWarrantyPolicy(l.ctx, &catalogPb.UpsertWarrantyPolicyReq{
		SkuId: parseID(req.Id), PeriodMonths: int32(req.PeriodMonths), StartRule: req.StartRule,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("catalog", "ListStationProducts", "catalog", "场站模板列表(perm: catalog:station:list)", '''func (l *ListStationProductsLogic) ListStationProducts(req *types.StationProductListReq) (resp *types.StationProductListResp, err error) {
	r, err := l.svcCtx.Catalog.ListStationProduct(l.ctx, &catalogPb.ListStationProductReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.StationProductListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.StationProductItem{
			StationProductID: strconv.FormatInt(s.StationProductId, 10), Name: s.Name,
			Remark: s.Remark, CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}''')

w("catalog", "GetStationProduct", "catalog", "场站模板详情(含 BOM,perm: catalog:station:list)", '''func (l *GetStationProductLogic) GetStationProduct(req *types.IDPath) (resp *types.StationProductDetailResp, err error) {
	r, err := l.svcCtx.Catalog.GetStationProduct(l.ctx, &catalogPb.GetStationProductReq{StationProductId: parseID(req.Id)})
	if err != nil {
		return nil, err
	}
	sp := r.StationProduct
	out := &types.StationProductDetailResp{StationProduct: types.StationProductItem{
		StationProductID: strconv.FormatInt(sp.StationProductId, 10), Name: sp.Name,
		Remark: sp.Remark, CreatedAt: sp.CreatedAt,
	}}
	for _, it := range sp.Items {
		out.StationProduct.Items = append(out.StationProduct.Items, types.BomItem{
			SkuID: strconv.FormatInt(it.SkuId, 10), SkuName: it.SkuName, Qty: int(it.Qty),
		})
	}
	return out, nil
}''')

w("catalog", "CreateStationProduct", "catalog", "新建场站模板(perm: catalog:station:create)", '''func (l *CreateStationProductLogic) CreateStationProduct(req *types.StationProductCreateReq) (resp *types.SimpleResp, err error) {
	items := make([]*catalogPb.BomItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &catalogPb.BomItem{SkuId: parseID(it.SkuID), Qty: int32(it.Qty)})
	}
	_, err = l.svcCtx.Catalog.CreateStationProduct(l.ctx, &catalogPb.CreateStationProductReq{
		Name: req.Name, Remark: req.Remark, Items: items,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

# ---------- inventory ----------
def stock_op(group, name, rpc_method, comment):
    w(group, name, "inventory", comment, '''func (l *%sLogic) %s(req *types.StockOpReq) (resp *types.StockOpResp, err error) {
	bizNo := req.BizNo
	if bizNo == "" {
		bizNo = "ADMIN" + strconv.FormatInt(ctxkit.UID(l.ctx), 10) + time.Now().Format("150405.000000000")
	}
	r, err := l.svcCtx.Inventory.%s(l.ctx, &inventoryPb.%s{
		WarehouseId: parseID(req.WarehouseID), SkuId: parseID(req.SkuID), Qty: int32(req.Qty),
		BizType: opBizType(req.BizType), BizNo: bizNo, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.StockOpResp{RecordID: strconv.FormatInt(r.RecordId, 10)}, nil
}''' % (name, name, rpc_method, rpc_method))

w("inventory", "ListWarehouses", "inventory", "仓库列表(perm: inventory:warehouse:list)", '''func (l *ListWarehousesLogic) ListWarehouses(req *types.WarehouseListReq) (resp *types.WarehouseListResp, err error) {
	r, err := l.svcCtx.Inventory.ListWarehouse(l.ctx, &inventoryPb.ListWarehouseReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.WarehouseListResp{Total: r.Total}
	for _, w := range r.List {
		resp.List = append(resp.List, types.WarehouseItem{
			WarehouseID: strconv.FormatInt(w.WarehouseId, 10), Code: w.Code, Name: w.Name,
			Address: w.Address, Status: int(w.Status), CreatedAt: w.CreatedAt,
		})
	}
	return resp, nil
}''')

w("inventory", "CreateWarehouse", "inventory", "新建仓库(perm: inventory:warehouse:create)", '''func (l *CreateWarehouseLogic) CreateWarehouse(req *types.WarehouseCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Inventory.CreateWarehouse(l.ctx, &inventoryPb.CreateWarehouseReq{Code: req.Code, Name: req.Name, Address: req.Address})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("inventory", "ListInventory", "inventory", "库存列表(四态,perm: inventory:inventory:list)", '''func (l *ListInventoryLogic) ListInventory(req *types.InventoryListReq) (resp *types.InventoryListResp, err error) {
	r, err := l.svcCtx.Inventory.ListInventory(l.ctx, &inventoryPb.ListInventoryReq{
		WarehouseId: parseID(req.WarehouseID), SkuId: parseID(req.SkuID), LowStockOnly: req.LowOnly,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InventoryListResp{Total: r.Total}
	for _, i := range r.List {
		resp.List = append(resp.List, types.InventoryItem{
			InventoryID: strconv.FormatInt(i.InventoryId, 10), WarehouseID: strconv.FormatInt(i.WarehouseId, 10),
			SkuID: strconv.FormatInt(i.SkuId, 10), Available: int(i.Available), Locked: int(i.Locked),
			InTransit: int(i.InTransit), Defective: int(i.Defective),
			LowStockThreshold: int(i.LowStockThreshold), UpdatedAt: i.UpdatedAt,
		})
	}
	return resp, nil
}''')

stock_op("inventory", "StockIn", "StockIn", "入库(perm: inventory:stock:in)")
stock_op("inventory", "ReserveStock", "Reserve", "预留(perm: inventory:stock:reserve)")
stock_op("inventory", "ReleaseStock", "Release", "释放(perm: inventory:stock:release)")
stock_op("inventory", "DeductStock", "DeductLocked", "出库发货(perm: inventory:stock:deduct)")
stock_op("inventory", "SpareOut", "SpareOut", "备件领用(perm: inventory:stock:spare-out)")
stock_op("inventory", "SpareReturn", "SpareReturn", "备件退库(perm: inventory:stock:spare-return)")

w("inventory", "ListStockRecords", "inventory", "库存流水(perm: inventory:record:list)", '''func (l *ListStockRecordsLogic) ListStockRecords(req *types.StockRecordListReq) (resp *types.StockRecordListResp, err error) {
	r, err := l.svcCtx.Inventory.ListStockRecord(l.ctx, &inventoryPb.ListStockRecordReq{
		WarehouseId: parseID(req.WarehouseID), SkuId: parseID(req.SkuID), BizType: req.BizType,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.StockRecordListResp{Total: r.Total}
	for _, rec := range r.List {
		resp.List = append(resp.List, types.StockRecordItem{
			RecordID: strconv.FormatInt(rec.RecordId, 10), InventoryID: strconv.FormatInt(rec.InventoryId, 10),
			WarehouseID: strconv.FormatInt(rec.WarehouseId, 10), SkuID: strconv.FormatInt(rec.SkuId, 10),
			BizType: rec.BizType, BizNo: rec.BizNo, Qty: int(rec.Qty),
			BeforeAvailable: int(rec.BeforeAvailable), AfterAvailable: int(rec.AfterAvailable),
			BeforeLocked: int(rec.BeforeLocked), AfterLocked: int(rec.AfterLocked),
			Remark: rec.Remark, CreatedAt: rec.CreatedAt,
		})
	}
	return resp, nil
}''')

w("inventory", "ListStocktakes", "inventory", "盘点单列表(perm: inventory:stocktake:list)", '''func (l *ListStocktakesLogic) ListStocktakes(req *types.StocktakeListReq) (resp *types.StocktakeListResp, err error) {
	r, err := l.svcCtx.Inventory.ListStocktake(l.ctx, &inventoryPb.ListStocktakeReq{
		WarehouseId: parseID(req.WarehouseID), Status: int32(req.Status), Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.StocktakeListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.StocktakeRecord{
			StocktakeID: strconv.FormatInt(s.StocktakeId, 10), WarehouseID: strconv.FormatInt(s.WarehouseId, 10),
			Status: int(s.Status), Remark: s.Remark, CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}''')

w("inventory", "CreateStocktake", "inventory", "创建盘点单(快照账面,perm: inventory:stocktake:create)", '''func (l *CreateStocktakeLogic) CreateStocktake(req *types.StocktakeCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Inventory.CreateStocktake(l.ctx, &inventoryPb.CreateStocktakeReq{
		WarehouseId: parseID(req.WarehouseID), Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("inventory", "GetStocktake", "inventory", "盘点单详情(perm: inventory:stocktake:list)", '''func (l *GetStocktakeLogic) GetStocktake(req *types.IDPath) (resp *types.StocktakeDetailResp, err error) {
	r, err := l.svcCtx.Inventory.GetStocktake(l.ctx, &inventoryPb.GetStocktakeReq{StocktakeId: parseID(req.Id)})
	if err != nil {
		return nil, err
	}
	s := r.Stocktake
	out := &types.StocktakeDetailResp{Stocktake: types.StocktakeRecord{
		StocktakeID: strconv.FormatInt(s.StocktakeId, 10), WarehouseID: strconv.FormatInt(s.WarehouseId, 10),
		Status: int(s.Status), Remark: s.Remark, CreatedAt: s.CreatedAt,
	}}
	for _, it := range s.Items {
		out.Stocktake.Items = append(out.Stocktake.Items, types.StocktakeItem{
			ItemID: strconv.FormatInt(it.ItemId, 10), SkuID: strconv.FormatInt(it.SkuId, 10),
			BookQty: int(it.BookQty), CountedQty: int(it.CountedQty), DiffQty: int(it.DiffQty),
		})
	}
	return out, nil
}''')

w("inventory", "SubmitStocktake", "inventory", "提交实盘(perm: inventory:stocktake:submit)", '''func (l *SubmitStocktakeLogic) SubmitStocktake(req *types.StocktakeSubmitReq) (resp *types.SimpleResp, err error) {
	items := make([]*inventoryPb.SubmitStocktakeCountItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &inventoryPb.SubmitStocktakeCountItem{SkuId: parseID(it.SkuID), CountedQty: int32(it.CountedQty)})
	}
	_, err = l.svcCtx.Inventory.SubmitStocktakeCount(l.ctx, &inventoryPb.SubmitStocktakeCountReq{
		StocktakeId: parseID(req.Id), Items: items,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("inventory", "ApproveStocktake", "inventory", "盘点审批(通过生成账面调整流水,perm: inventory:stocktake:approve)", '''func (l *ApproveStocktakeLogic) ApproveStocktake(req *types.StocktakeApproveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Inventory.ApproveStocktake(l.ctx, &inventoryPb.ApproveStocktakeReq{
		StocktakeId: parseID(req.Id), Approve: req.Approve, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

# ---------- notification ----------
w("notification", "ListMessages", "notification", "消息列表(perm: notification:message:list)", '''func (l *ListMessagesLogic) ListMessages(req *types.MessageListReq) (resp *types.MessageListResp, err error) {
	uid := parseID(req.UserID)
	if uid == 0 {
		uid = ctxkit.UID(l.ctx)
	}
	r, err := l.svcCtx.Notification.ListMessages(l.ctx, &notificationPb.ListMessagesReq{
		UserId: uid, OnlyUnread: req.OnlyUnread, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.MessageListResp{Total: r.Total}
	for _, m := range r.List {
		resp.List = append(resp.List, types.MessageItem{
			MessageID: strconv.FormatInt(m.MessageId, 10), UserID: strconv.FormatInt(m.UserId, 10),
			Title: m.Title, Content: m.Content, IsRead: m.IsRead,
			BizType: m.BizType, BizID: m.BizId, CreatedAt: m.CreatedAt,
		})
	}
	return resp, nil
}''')

w("notification", "UnreadCount", "notification", "未读数(perm: notification:message:list)", '''func (l *UnreadCountLogic) UnreadCount() (resp *types.UnreadCountResp, err error) {
	r, err := l.svcCtx.Notification.UnreadCount(l.ctx, &notificationPb.UnreadCountReq{UserId: ctxkit.UID(l.ctx)})
	if err != nil {
		return nil, err
	}
	return &types.UnreadCountResp{Count: r.Count}, nil
}''')

w("notification", "MarkRead", "notification", "标记已读(perm: notification:message:read)", '''func (l *MarkReadLogic) MarkRead(req *types.IDPath) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Notification.MarkRead(l.ctx, &notificationPb.MarkReadReq{
		MessageId: parseID(req.Id), UserId: ctxkit.UID(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("notification", "ListTemplates", "notification", "模板列表(perm: notification:template:list)", '''func (l *ListTemplatesLogic) ListTemplates(req *types.TemplateListReq) (resp *types.TemplateListResp, err error) {
	r, err := l.svcCtx.Notification.ListTemplate(l.ctx, &notificationPb.ListTemplateReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.TemplateListResp{Total: r.Total}
	for _, t := range r.List {
		resp.List = append(resp.List, types.TemplateItem{
			TemplateID: strconv.FormatInt(t.TemplateId, 10), Code: t.Code,
			TitleTemplate: t.TitleTemplate, ContentTemplate: t.ContentTemplate,
			Channel: t.Channel, Status: int(t.Status),
		})
	}
	return resp, nil
}''')

w("notification", "UpsertTemplate", "notification", "模板 Upsert(perm: notification:template:update)", '''func (l *UpsertTemplateLogic) UpsertTemplate(req *types.TemplateUpsertReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Notification.UpsertTemplate(l.ctx, &notificationPb.UpsertTemplateReq{
		Code: req.Code, TitleTemplate: req.TitleTemplate, ContentTemplate: req.ContentTemplate,
		Channel: req.Channel, Status: int32(req.Status),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

w("notification", "ListNotifySettings", "notification", "我的通知设置(perm: notification:setting:list)", '''func (l *ListNotifySettingsLogic) ListNotifySettings() (resp *types.NotifySettingListResp, err error) {
	r, err := l.svcCtx.Notification.GetUserSetting(l.ctx, &notificationPb.GetUserSettingReq{UserId: ctxkit.UID(l.ctx)})
	if err != nil {
		return nil, err
	}
	resp = &types.NotifySettingListResp{}
	for _, s := range r.List {
		resp.List = append(resp.List, types.NotifySettingItem{
			UserID: strconv.FormatInt(s.UserId, 10), TemplateCode: s.TemplateCode,
			Enabled: s.Enabled, QuietHours: s.QuietHours,
		})
	}
	return resp, nil
}''')

w("notification", "UpsertNotifySetting", "notification", "通知设置 Upsert(perm: notification:setting:update)", '''func (l *UpsertNotifySettingLogic) UpsertNotifySetting(req *types.NotifySettingUpsertReq) (resp *types.SimpleResp, err error) {
	uid := parseID(req.UserID)
	if uid == 0 {
		uid = ctxkit.UID(l.ctx)
	}
	_, err = l.svcCtx.Notification.UpdateUserSetting(l.ctx, &notificationPb.UpdateUserSettingReq{
		UserId: uid, TemplateCode: req.TemplateCode, Enabled: req.Enabled, QuietHours: req.QuietHours,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}''')

# ---------- audit ----------
w("audit", "ListAuditLogs", "audit", "操作审计(perm: audit:log:list)", '''func (l *ListAuditLogsLogic) ListAuditLogs(req *types.AuditLogListReq) (resp *types.AuditLogListResp, err error) {
	r, err := l.svcCtx.Audit.ListLog(l.ctx, &auditPb.ListLogReq{
		Action: req.Action, Uid: parseID(req.UID), TargetType: req.TargetType, TargetId: req.TargetID,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.AuditLogListResp{Total: r.Total}
	for _, a := range r.List {
		resp.List = append(resp.List, types.AuditLogItem{
			LogID: strconv.FormatInt(a.LogId, 10), TraceID: a.TraceId, UID: strconv.FormatInt(a.Uid, 10),
			Action: a.Action, TargetType: a.TargetType, TargetID: a.TargetId,
			BeforeJson: a.BeforeJson, AfterJson: a.AfterJson, Result: a.Result,
			Client: a.Client, OnBehalfOf: strconv.FormatInt(a.OnBehalfOf, 10), CreatedAt: a.CreatedAt,
		})
	}
	return resp, nil
}''')

w("audit", "ListCmdLogs", "audit", "指令审计(perm: audit:cmd:list)", '''func (l *ListCmdLogsLogic) ListCmdLogs(req *types.CmdAuditListReq) (resp *types.CmdAuditListResp, err error) {
	r, err := l.svcCtx.Audit.ListCmdLog(l.ctx, &auditPb.ListCmdLogReq{
		Sn: req.SN, CmdId: req.CmdID, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.CmdAuditListResp{Total: r.Total}
	for _, a := range r.List {
		resp.List = append(resp.List, types.CmdAuditItem{
			CmdAuditID: strconv.FormatInt(a.CmdAuditId, 10), CmdID: a.CmdId, SN: a.Sn,
			UID: strconv.FormatInt(a.Uid, 10), Action: a.Action, PayloadJson: a.PayloadJson,
			Result: a.Result, Error: a.Error, TraceID: a.TraceId, CreatedAt: a.CreatedAt,
		})
	}
	return resp, nil
}''')

print("generated", len(os.listdir(os.path.join(ROOT, "party"))), "party logic files")
