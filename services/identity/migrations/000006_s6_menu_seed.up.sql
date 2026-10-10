-- S6 资产与 IoT 菜单集（S6-01/02 /auth/menus 数据源 + 前端页面路由源）。
-- 结构对齐 02 §6.2：目录(1)/菜单(2)/按钮(3)。固定 ID（9.0e18 段，S6 段 9000000000000000601+）；
-- 重复执行幂等（INSERT IGNORE）；role_menu 同步绑定平台管理员角色（role_code=admin，seed 种入）。
-- perm_code 与 admin-bff perm_resolver / 前端 <Perm code> 严格同源。

INSERT IGNORE INTO `menu`
    (`menu_id`,`parent_id`,`name`,`type`,`perm_code`,`path`,`icon`,`sort`,`status`,`tenant_id`,`created_at`)
VALUES
    -- 资产中心目录
    (9000000000000000601, 0, '资产中心', 1, NULL, '/asset', 'cluster', 6, 1, 9000000000000000001, NOW()),
    (9000000000000000611, 9000000000000000601, '设备管理', 2, 'asset:device:list', '/asset/devices', 'usb', 1, 1, 9000000000000000001, NOW()),
    (9000000000000000612, 9000000000000000611, '设备导入', 3, 'asset:device:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000613, 9000000000000000611, '设备激活', 3, 'asset:device:activate', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000614, 9000000000000000611, '状态迁移', 3, 'asset:device:transition', NULL, NULL, 3, 1, 9000000000000000001, NOW()),
    (9000000000000000615, 9000000000000000611, '凭证补发', 3, 'asset:device:credential', NULL, NULL, 4, 1, 9000000000000000001, NOW()),
    (9000000000000000616, 9000000000000000611, '指令下发', 3, 'asset:device:cmd', NULL, NULL, 5, 1, 9000000000000000001, NOW()),
    (9000000000000000617, 9000000000000000611, '拓扑维护', 3, 'asset:device:topology', NULL, NULL, 6, 1, 9000000000000000001, NOW()),
    (9000000000000000621, 9000000000000000601, 'OTA 任务', 2, 'asset:ota:task', '/asset/ota-tasks', 'cloud-upload', 2, 1, 9000000000000000001, NOW()),
    (9000000000000000622, 9000000000000000621, '固件登记', 3, 'asset:ota:firmware', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000623, 9000000000000000621, '任务回滚', 3, 'asset:ota:rollback', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000631, 9000000000000000601, '场站管理', 2, 'asset:station:list', '/asset/stations', 'home', 3, 1, 9000000000000000001, NOW()),
    (9000000000000000632, 9000000000000000631, '场站创建', 3, 'asset:station:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000633, 9000000000000000631, '设备绑定', 3, 'asset:station:bind', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000634, 9000000000000000631, '拓扑维护', 3, 'asset:station:topology', NULL, NULL, 3, 1, 9000000000000000001, NOW()),
    (9000000000000000635, 9000000000000000631, '驻场人员', 3, 'asset:station:staff', NULL, NULL, 4, 1, 9000000000000000001, NOW()),
    (9000000000000000641, 9000000000000000601, '指令记录', 2, 'asset:device:cmd', '/asset/commands', 'thunderbolt', 4, 1, 9000000000000000001, NOW());

-- 平台管理员角色绑定全部新菜单（role_code=admin，seed 9000000000000000003）。
INSERT IGNORE INTO `role_menu`
    (`role_id`,`menu_id`,`tenant_id`,`created_at`)
SELECT 9000000000000000003, m.`menu_id`, 9000000000000000001, NOW()
FROM `menu` m
WHERE m.`menu_id` >= 9000000000000000601 AND m.`menu_id` <= 9000000000000000641
  AND m.`tenant_id` = 9000000000000000001;
