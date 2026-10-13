-- 修复 S4 菜单迁移的角色绑定缺口：000004 起给 admin 绑定用 menu_id >= 9000000000000000201
-- 范围过滤，漏掉 000003 的 system:* 段（用户管理等系统菜单，admin 无绑定则 BFF 侧 10408）。
-- 口径：role.code='admin' × 同租户全部未软删菜单全量绑定（INSERT IGNORE 幂等，PK role_id+menu_id），
-- 同时兜底未来菜单 seed 漏绑角色的情况。
INSERT IGNORE INTO `role_menu` (`role_id`, `menu_id`, `tenant_id`, `created_at`)
SELECT r.`role_id`, m.`menu_id`, m.`tenant_id`, NOW()
FROM `role` r
JOIN `menu` m ON m.`tenant_id` = r.`tenant_id`
WHERE r.`code` = 'admin'
  AND m.`deleted_at` IS NULL;
