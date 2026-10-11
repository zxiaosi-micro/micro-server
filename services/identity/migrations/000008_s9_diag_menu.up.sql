-- S9 运维 PC 本地诊断菜单（S9-05 /auth/menus 数据源；Tauri 内可用，浏览器占位拒绝）。
-- 固定 ID 沿 9.0e18 段（S9 起用 9000000000000000801+）；INSERT IGNORE 幂等；admin 角色全绑。

INSERT IGNORE INTO `menu`
    (`menu_id`,`parent_id`,`name`,`type`,`perm_code`,`path`,`icon`,`sort`,`status`,`tenant_id`,`created_at`)
VALUES
    (9000000000000000801, 0, '本地诊断', 2, 'ops:diag:local', '/diag', 'tool', 10, 1, 9000000000000000001, NOW());

-- 平台管理员角色绑定（role_code=admin，seed 9000000000000000003）。
INSERT IGNORE INTO `role_menu`
    (`role_id`,`menu_id`,`tenant_id`,`created_at`)
VALUES (9000000000000000003, 9000000000000000801, 9000000000000000001, NOW());
