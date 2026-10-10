-- analytics_db 000001 回滚。
DROP TABLE IF EXISTS `event_outbox`;
DROP TABLE IF EXISTS `recon_state`;
DROP TABLE IF EXISTS `recon_diff`;
DROP TABLE IF EXISTS `export_task`;
DROP TABLE IF EXISTS `report_meta`;
DROP TABLE IF EXISTS `report_watermark`;
DROP TABLE IF EXISTS `report_station_daily`;
DROP TABLE IF EXISTS `report_finance_daily`;
DROP TABLE IF EXISTS `report_sla_daily`;
DROP TABLE IF EXISTS `report_warranty_daily`;
DROP TABLE IF EXISTS `report_inventory_daily`;
DROP TABLE IF EXISTS `report_sales_daily`;
DROP TABLE IF EXISTS `report_event_entry`;
