-- S4 菜单 seed 回滚：逆序删除 role_menu 绑定与 9.0e18 201+ 段菜单。
DELETE FROM `role_menu` WHERE `menu_id` >= 9000000000000000201 AND `menu_id` <= 9000000000000000321;
DELETE FROM `menu` WHERE `menu_id` >= 9000000000000000201 AND `menu_id` <= 9000000000000000321;
