-- notification_db 000002 回滚：逆序 DROP。
DROP TABLE IF EXISTS `event_dead`;
DROP TABLE IF EXISTS `event_retry`;
DROP TABLE IF EXISTS `event_dedup`;
