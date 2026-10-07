-- identity_db 初始 8 表回滚（S3-01，与 000001_init.up.sql 成对；逆序 DROP）
DROP TABLE IF EXISTS `user_sso_binding`;
DROP TABLE IF EXISTS `role_menu`;
DROP TABLE IF EXISTS `user_role`;
DROP TABLE IF EXISTS `menu`;
DROP TABLE IF EXISTS `role`;
DROP TABLE IF EXISTS `org`;
DROP TABLE IF EXISTS `user`;
DROP TABLE IF EXISTS `tenant`;
