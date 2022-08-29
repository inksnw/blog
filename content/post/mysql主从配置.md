---
title: "Mysql主从配置"
date: 2022-08-09T09:10:34+08:00
tags: ["数据库"]
---



安装三个数据库

```bash
for i in $(seq 1 3)
do
name=mysql"$i"
port=330"$i"
mkdir "$name"
docker run -p "$port":3306 --name "$name" -v /root/"$name"/conf:/etc/mysql/conf.d -v /root/"$name"/logs:/var/log/mysql -v /root/"$name"/data:/var/lib/mysql -e MYSQL_ROOT_PASSWORD=123456 -d mysql:5.7 --character-set-server=utf8mb4
done

```
开启bin_log
```sql
[mysqld]
# 数据同步时数据库唯一标识，server_id参数的值必须是数字，否则启动MySQL会报错
server-id=132

# 开启二进制日志
log-bin=/var/lib/mysql/mysql-bin
```
确认开启成功
```sql
show variables like '%log_bin%'
log_bin	ON
log_bin_basename	/var/lib/mysql/mysql-bin
log_bin_index	/var/lib/mysql/mysql-bin.index
log_bin_trust_function_creators	OFF
log_bin_use_v1_row_events	OFF
sql_log_bin	ON
```

开启主从

```sql
CHANGE MASTER TO MASTER_HOST='192.168.50.231', MASTER_USER='root',MASTER_PORT=3301, MASTER_PASSWORD='123456', MASTER_LOG_FILE='mysql-bin.000001', MASTER_LOG_POS=154;
start slave;
show slave status;
-- 关闭主从命令
stop slave;
reset slave all;
-- 验证主从状态
-- Slave_IO_Running和Slave_SQL_Running都为YES的时候就表示主从同步设置成功了
```

验证

在主库上创建表,写入数据,查看从库

```sql
create database test default character set utf8mb4 collate utf8mb4_unicode_ci;

DROP TABLE IF EXISTS `student`;
CREATE TABLE `student` (
  `test` varchar(255) NOT NULL,
  PRIMARY KEY (`test`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

BEGIN;
INSERT INTO `student` VALUES ('aaa');
COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
```

变更为半同步

```sql
# master服务器执行命令：
INSTALL PLUGIN rpl_semi_sync_master SONAME 'semisync_master.so';
# slave服务器执行命令：
INSTALL PLUGIN rpl_semi_sync_slave SONAME 'semisync_slave.so';

# 查看插件是否安装成功
show plugins;
```

1. 修改/etc/my.cnf文件，在[mysqld]下添加如下配置：

2. master服务器添加配置

```bash
plugin-load="rpl_semi_sync_master=semisync_master.so"
rpl-semi-sync-master-enabled = 1
```

3. slave服务器添加配置

```bash
plugin-load="rpl_semi_sync_slave=semisync_slave.so"
rpl_semi_sync_slave_enabled=1
```

重启数据库

```sql
-- 重启数据库后查看master
show STATUS like '%rpl_semi_sync_master_status%'
-- Rpl_semi_sync_master_status	ON
-- 查看从库状态
show STATUS like '%rpl_semi_sync_master_status%'
-- Rpl_semi_sync_slave_status  ON
```

写入数据验证

主库写入数据,从库同时出现

