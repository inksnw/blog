---
title: "Prometheus简单使用"
date: 2022-08-24T15:11:03+08:00
tags: ["k8s"]
---

## 安装prometheus operator

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
kubectl create ns monitor
helm install myprometheus prometheus-community/kube-prometheus-stack -n monitor
```
报错解决

> plugin type="calico" failed (add): error getting ClusterInformation: connection is unauthorized: Unauthorized

删除 /etc/cni/net.d/下的文件,重启后会重建,暂还不清楚原因

会同时安装以下组件

- [prometheus-community/kube-state-metrics](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics)
- [prometheus-community/prometheus-node-exporter](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus-node-exporter)
- [grafana/grafana](https://github.com/grafana/helm-charts/tree/main/charts/grafana)

## 安装prometheus-adapter (可选)

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install myadapter prometheus-community/prometheus-adapter -n monitor
```

## 安装metrics-server (可选)

> 报错 **`no metrics known for pod`** `x509: cannot validate certificate`。

给部署添加如下命令行即可：

```bash

args:
  - --kubelet-preferred-address-types=InternalIP
  - --kubelet-insecure-tls
```

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

## 创建一个nginx服务

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mynginx
spec:
  selector:
    app: mynginx
  ports:
    - port: 80
      targetPort: 80
  type: NodePort
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mynginx
  labels:
    app: mynginx
spec:
  replicas: 1
  template:
    metadata:
      name: mynginx
      labels:
        app: mynginx
    spec:
      containers:
        - name: mynginx
          image: nginx
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 80
          volumeMounts:
            - name: web-nginx-config
              mountPath: /etc/nginx/nginx.conf
              subPath: nginx.conf
      volumes:
        - name: web-nginx-config
          configMap:
            name: web-nginx-config
            items:
              - key: nginx.conf
                path: nginx.conf
      restartPolicy: Always
  selector:
    matchLabels:
      app: mynginx

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: web-nginx-config
data:
  nginx.conf: |
    user  nginx;
    worker_processes  auto;
    error_log  /var/log/nginx/error.log notice;
    pid        /var/run/nginx.pid;
    events {
        worker_connections  1024;
    }
    http {
        include       /etc/nginx/mime.types;
        default_type  application/octet-stream;
        log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                          '$status $body_bytes_sent "$http_referer" '
                          '"$http_user_agent" "$http_x_forwarded_for"';
        access_log  /var/log/nginx/access.log  main;
        sendfile        on;
        keepalive_timeout  65;
        server {
          listen       80;
          listen  [::]:80;
          server_name  localhost;
          location / {
              root   /usr/share/nginx/html;
              index  index.html index.htm;
          }
          location /status {
            stub_status;
          }
          error_page   500 502 503 504  /50x.html;
          location = /50x.html {
              root   /usr/share/nginx/html;
          }
      }
    }
```

访问status

```
http://192.168.50.40:30509/status
```

![image-20220919014046228](http://inksnw.asuscomm.com:3001/blog/prometheus_59c00197e79cd0cdb7c4e4c68e18f6fc.png)

> Active connections: 当前nginx正在处理的活动连接数.
>
> Server accepts handled requests: 
>  nginx启动以来总共处理了10个连接
>  成功创建10握手(证明中间没有失败的)
>  总共处理了12个请求。
>
> Reading: nginx读取到客户端的Header信息数.
> Writing: nginx返回给客户端的Header信息数.
> Waiting: 开启keep-alive的情况下,这个值等于 active – (reading + writing),意思就是nginx已经处理完成,正在等候下一次请求指令的驻留连接。
> 所以,在访问效率高,请求很快被处理完毕的情况下,Waiting数比较多是正常的.如果reading +writing数较多,则说明并发访问量非常大,正在处理过程中。

## 创建metrics服务

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-prometheus-exporter
spec:
  selector:
    matchLabels:
      k8s: mynginx-exporter
  template:
    metadata:
      labels:
        k8s: mynginx-exporter
    spec:
      containers:
        - name: mynginx-exporter
          image: nginx/nginx-prometheus-exporter
          imagePullPolicy: IfNotPresent
          command:
            - "nginx-prometheus-exporter"
            - "-nginx.scrape-uri=http://mynginx/status"
---
kind: Service
apiVersion: v1
metadata:
  name: nginx-prometheus-exporter
  labels:
    k8s: mynginx-exporter
spec:
  ports:
    - port: 9113
      targetPort: 9113
      name: metrics-port
      protocol: TCP
  type: NodePort
  selector:
    k8s: mynginx-exporter
```

> metadata.labels: 这下的标签要和ServiceMonitor的 spec.selector.matchLabels: 选择的标签一致。



访问测试

```
http://192.168.50.40:32127/metrics
```

![Snipaste_2022-09-19_01-20-30](http://inksnw.asuscomm.com:3001/blog/prometheus_b4e0607fb2e115113f0f97a75ef72387.png)

## 创建ServiceMonitor

一个业务pod通过exporter生成metrics svc,再由ServiceMonitor通过标签和port名筛选找到,operator会把这个信息转化成相应的prometheus配置文件

<img src="http://inksnw.asuscomm.com:3001/blog/prometheus_e8a23cc25c96c0bc0f620b7f48fd4208.png" alt="prometheus-architecture" style="zoom:50%;" />

```yaml
kind: ServiceMonitor
apiVersion: monitoring.coreos.com/v1
metadata:
  name: mynginx-monitor
spec:
  endpoints:
    - interval: 3s
      port: metrics-port
  selector:
    matchLabels:
      k8s: mynginx-exporter
```

> 关键词注释：
>
> spec.endpoints.interval：30s 每隔30秒获取一次信息
>
> spec.endpoints.port: http-metrics:  对应Service的端口名称，在service资源中的spec.ports.name中定义。
>
> spec.namespaceSelector.matchNames:  匹配某名称空间的service，如果要从所有名称空间中匹配用any: true
>
> spec.selector.matchLabels:  匹配Service的标签，多个标签为“与”关系
>
> spec.selector.matchExpressions:  匹配Service的标签，多个标签是“或”关系

转发服务

```bash
kubectl port-forward svc/prometheus-k8s 9090:9090 -n kubesphere-monitoring-system --address="0.0.0.0"
```

### 查看服务发现

![Snipaste_2022-09-19_01-31-48](http://inksnw.asuscomm.com:3001/blog/prometheus_caf8d8233205d8c93e40a9a12d54ecec.png)

### 查询数据

![image-20220919022556769](http://inksnw.asuscomm.com:3001/blog/prometheus_a610ae00f28947f25fcba182f52bcc32.png)

## 创建一个告警规则

参考

- https://segmentfault.com/a/1190000040639025

- https://www.qikqiak.com/post/prometheus-operator-custom-alert/

传统的prometheus单进程部署模式下，我们如何定义报警规则：

1. 修改配置文件prometheus.yaml，增加报警规则定义；
2. POST /-/reload让配置生效；

在prometheus-operator部署模式下，我们仅需定义prometheusrule资源对象即可，operator监听到prometheusrule资源对象被创建，会自动为我们添加告警规则文件，自动reload。

### 默认的告警规则

prometheus-operator部署出来的prometheus默认已经有一些规则，在prometheus-k8s-0这个pod的目录下面：

```bash
$ ls /etc/prometheus/rules/prometheus-k8s-rulefiles-0/
{namespace}-prometheus-k8s-rules.yaml

```
### 创建prometheurule资源对象

Prometheus通过配置来匹配 alertmanager的endpoints来关联alertmanager。添加自定义监控需要创建PrometheusRule资源，并且要包含prometheus=k8s和 role=alert-rules这两个标签，因为Prometheus通过这两个标签选择此资源对象。

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  labels:
    prometheus: k8s
    role: alert-rules
  name: myalert
spec:
  groups:
    - name: myalertgroup
      rules:
        - alert: myalert1
          annotations:
            aliasName: ""
            description: ""
            message: '当前值：{{ $value }}'
            rule_update_time: "2022-09-15T01:20:00Z"
            summary: 无法提供服务
          expr: sum(nginx_connections_accepted) > 10
          for: 1m
          labels:
            severity: error
```

### 查看是否生效

![image-20220919025117742](http://inksnw.asuscomm.com:3001/blog/prometheus_c096e7974f4018b110d8693cc717b8a8.png)

触发告警

![image-20220919025150230](http://inksnw.asuscomm.com:3001/blog/prometheus_349a5dd3f228633614cc4c09a862e63b.png)
