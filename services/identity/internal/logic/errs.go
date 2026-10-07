// identity 业务错误码（SegIdentity 段内偏移 1000+；认证族 400~408 已由公共库占用）。

package logic

import "github.com/zxiaosi-micro/micro-common/errcode"

var (
	errMobileUsed       = errcode.New(errcode.SegIdentity, 1001, "手机号已被使用")
	errEmailUsed        = errcode.New(errcode.SegIdentity, 1002, "邮箱已被使用")
	errUserNotFound     = errcode.New(errcode.SegIdentity, 1003, "用户不存在")
	errRoleNotFound     = errcode.New(errcode.SegIdentity, 1004, "角色不存在")
	errOrgNotFound      = errcode.New(errcode.SegIdentity, 1005, "组织不存在")
	errMenuNotFound     = errcode.New(errcode.SegIdentity, 1006, "菜单不存在")
	errTenantNotFound   = errcode.New(errcode.SegIdentity, 1007, "租户不存在")
	errRoleCodeUsed     = errcode.New(errcode.SegIdentity, 1008, "角色编码已存在")
	errPermCodeUsed     = errcode.New(errcode.SegIdentity, 1009, "权限码已存在")
	errHasChildren      = errcode.New(errcode.SegIdentity, 1010, "存在子节点,不可删除")
	errRoleHasUsers     = errcode.New(errcode.SegIdentity, 1011, "角色下存在用户,不可删除")
	errTenantCodeUsed   = errcode.New(errcode.SegIdentity, 1012, "租户编码已存在")
	errWechatNotConf    = errcode.New(errcode.SegIdentity, 1013, "微信登录未配置")
	errOrgHasUsers      = errcode.New(errcode.SegIdentity, 1014, "组织下存在用户,不可删除")
	errTenantHasUsers   = errcode.New(errcode.SegIdentity, 1015, "租户下存在用户,不可删除")
	errClientNotAllowed = errcode.New(errcode.SegIdentity, 1016, "该账号未开通此端登录")
)
