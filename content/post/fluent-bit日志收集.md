---
title: "Fluent Bit日志收集"
date: 2022-09-28T00:31:21+08:00
tags: ["k8s"]
---

组件

- fluent-bit 负责解析及数据过滤
- fluentd 负责接收fluent-bit解析后的数据,发送到mq等

## 安装测试

文档: https://docs.fluentbit.io/manual/installation/getting-started-with-fluent-bit

```bash
helm repo add fluent https://fluent.github.io/helm-charts
helm upgrade --install fluent-bit fluent/fluent-bit
```

默认安装的日志收集配置文件在名为`fluent-bit`的cm中

```
kubectl get cm fluent-bit  -o yaml
```

修改日志输出到标准输出

```
kubectl edit cm fluent-bit
```

```yaml
  output.conf: |
    [OUTPUT]
        Name   stdout
        Match  *
```

发现修改cm后配置不会自动更新,手动删除pod后查看日志

```bash
kubectl logs fluent-bit-ctgrx -f
```

## 日志输出到ES中

简单安装es,下载二进制

```
https://www.elastic.co/cn/downloads/elasticsearch
https://www.elastic.co/cn/downloads/kibana
```

修改`lasticsearch-8.4.2/config/elasticsearch.yml`文件

```yaml
network.host: 0.0.0.0
http.port: 9200
xpack.security.enabled: false
```

取消 `kibana-8.4.2/config` 中的以下注释

```yaml
elasticsearch.hosts: ["http://localhost:9200"]
server.host: "0.0.0.0"
server.port: 5601
```

启动

```bash
./elasticsearch-8.4.2/bin/elasticsearch
./kibana-8.4.2/bin/kibana
```

访问**es** `http://127.0.0.1:9200/`

```json
{
    "name": "inksnwdeiMac.local",
    "cluster_name": "elasticsearch",
    "cluster_uuid": "sh0TKV6dRxaHb-QDWRg34A",
    "version": {
        "number": "8.4.2",
        "build_flavor": "default",
        "build_type": "tar",
        "build_hash": "89f8c6d8429db93b816403ee75e5c270b43a940a",
        "build_date": "2022-09-14T16:26:04.382547801Z",
        "build_snapshot": false,
        "lucene_version": "9.3.0",
        "minimum_wire_compatibility_version": "7.17.0",
        "minimum_index_compatibility_version": "7.0.0"
    },
    "tagline": "You Know, for Search"
}
```

访问**kinaba**

```
http://127.0.0.1:5601/app/dev_tools#/console
```

两年没用es,8.0的kinaba界面改的我都认不出来了(-_-)

### 配置ES输出

文档: https://docs.fluentbit.io/manual/pipeline/outputs/elasticsearch

```yaml
    [OUTPUT]
        Name es
        Match *
        Host 192.168.50.251
        Port 9200
        Suppress_Type_Name On
```

到目前`1.9版本`还没有热更新,手动删除

```bash
kubectl get pod  | grep fluent-bit | awk '{print $1}'   | xargs kubectl delete pod 
```

查看ES信息

```bash
http://127.0.0.1:9200/_cat/indices
http://127.0.0.1:9200/fluent-bit/_search
yellow open fluent-bit x1FyeUYwTT-WheIwiHzq6A 1 1 107 0 187.9kb 187.9kb
```

查询

<img src="http://inksnw.asuscomm.com:3001/blog/fluent-bit日志收集_1539f772172be7ccf3a9f07e30b0579d.png" alt="image-20220928002559025" style="zoom:50%;" />

