-- 回滚 000011：仅删除 admin 角色对 system:* 菜单的绑定（本迁移实际补入的部分；
-- 000004~000009 各段已绑定的业务菜单不受影响）。
DELETE rm
FROM `role_menu` rm
JOIN `menu` m ON m.`menu_id` = rm.`menu_id` AND m.`tenant_id` = rm.`tenant_id`
JOIN `role` r ON r.`role_id` = rm.`role_id`
WHERE r.`code` = 'admin'
  AND m.`perm_code` LIKE 'system:%';
