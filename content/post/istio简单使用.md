---
title: "Istio简单使用"
date: 2022-06-30T10:27:37+08:00
tags: ["k8s"]
subs: ["istio"]
---

## 安装

### 主程序

参考文档: https://istio.io/latest/zh/docs/setup/install/


```bash
➜ brew install istioctl
# 查看 istioctl 可安装的模式
➜ istioctl profile list
Istio configuration profiles:
    default
    demo
    empty
    minimal
    openshift
    preview
    remote
➜ istioctl install --set profile=demo
# 查看安装了什么
➜ kubectl get pod -n istio-system
# 修改 ingressgateway 为 NodePort 模式
kubectl edit svc istio-ingressgateway  -n istio-system
```

### 其它组件

```bash
# Kiali
➜ kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.14/samples/addons/kiali.yaml
# Prometheus
➜ kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.14/samples/addons/prometheus.yaml
# jaeger
➜ kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.14/samples/addons/jaeger.yaml
```

grafana

```bash
➜ kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.14/samples/addons/grafana.yaml
# 修改Kiali 添加 grafana地址, 重启pod （delete pod即可）
➜ kubectl edit cm kiali  -n istio-system
```

```yaml
external_services:
  grafana:
    url: http://grafana:3000
```

## 注入
手动注入方法
```bash
istioctl kube-inject -f xx.yaml | kubectl apply -f -
```

开启自动注入

```bash
➜ kubectl create ns myistio
#自动注入
➜ kubectl label ns myistio istio-injection=enabled
➜ kubectl get ns -L istio-injection
```

## 暴露kiali服务

```yaml
cat<<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: kiali-gateway
  namespace: myistio
spec:
  selector:
    istio: ingressgateway # use Istio default gateway implementation
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - kiali.inksnw.com
---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: kialiinfo
  namespace: myistio
spec:
  hosts:
    - kiali.inksnw.com
  gateways:
    - kiali-gateway
  http:
    - match:
        - uri:
            prefix: /
      route:
        - destination:
            host: kiali.istio-system.svc.cluster.local
            port:
              number: 20001
EOF
```

配置hosts访问

```bash
curl http://kiali.inksnw.com:32016/
```



## 示例

### 资源配置

```bash
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prodapi
  namespace: myistio
spec:
  selector:
    matchLabels:
      app: prod
  replicas: 1
  template:
    metadata:
      labels:
        app: prod
        version: v1
    spec:
      nodeName: node1
      containers:
        - name: prod
          image: alpine:3.12
          imagePullPolicy: IfNotPresent
          command: [ "/app/prods" ]
          env:
            - name: "release"
              value: "1"
          volumeMounts:
            - name: appdata
              mountPath: /app
          ports:
            - containerPort: 8080
      volumes:
        - name: appdata
          hostPath:
            path: /root/build
---
apiVersion: v1
kind: Service
metadata:
  name: prodsvc
  namespace: myistio
  labels:
    app: prod
spec:
  type: ClusterIP
  ports:
    - port: 80
      targetPort: 8080
  selector:
    app: prod

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviewapi
  namespace: myistio
spec:
  selector:
    matchLabels:
      app: reviews
  replicas: 1
  template:
    metadata:
      labels:
        app: reviews
    spec:
      nodeName: node1
      containers:
        - name: reviews
          image: alpine:3.12
          imagePullPolicy: IfNotPresent
          command: [ "/app/reviews" ]
          env:
            - name: "release"
              value: "1"
          volumeMounts:
            - name: appdata
              mountPath: /app
          ports:
            - containerPort: 8080
      volumes:
        - name: appdata
          hostPath:
            path: /root/build
---
apiVersion: v1
kind: Service
metadata:
  name: reviewsvc
  namespace: myistio
  labels:
    app: review
spec:
  type: ClusterIP
  ports:
    - port: 80
      targetPort: 8080
  selector:
    app: reviews

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prodapiv2
  namespace: myistio
spec:
  selector:
    matchLabels:
      app: prod
  replicas: 1
  template:
    metadata:
      labels:
        app: prod
        version: v2
    spec:
      nodeName: node1
      containers:
        - name: prod
          image: alpine:3.12
          imagePullPolicy: IfNotPresent
          command: [ "/app/prodsv2" ]
          env:
            - name: "release"
              value: "1"
          volumeMounts:
            - name: appdata
              mountPath: /app
          ports:
            - containerPort: 8080
      volumes:
        - name: appdata
          hostPath:
            path: /root/build
---
```

### Gateway

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: prods-gateway
  namespace: myistio
spec:
  selector:
    istio: ingressgateway # 通过label选择gateway 服务
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - prods.inksnw.com
EOF
```
### VirtualService

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: prodvs
  namespace: myistio
spec:
  hosts:
    - prods.inksnw.com
    - reviewsvc
  gateways:
    - prods-gateway
  http:
    - match:
        - uri:
            prefix: "/p"
#      故障注入
#      fault:
#        abort:
#          httpStatus: 500
#          percentage:
#            value: 100
      rewrite:
        uri: "/prods"
      route:
        - destination:
            host: prodsvc
            subset: v1svc
            port:
              number: 80
    - match:
        - uri:
            prefix: "/"

      route:
        - destination:
            host: reviewsvc
            port:
              number: 80
```

查看信息

```bash
➜ kubectl get gateway -n myistio
➜ kubectl get VirtualService -n myistio
```

在本机hosts添加

```
192.168.50.40 prods.inksnw.com
```

访问测试

```bash
#找到istio-ingressgateway对应的 NodePort 端口
➜ kubectl get svc -n istio-system istio-ingressgateway
➜ curl http://prods.inksnw.com:32016/prods/123
```
### DestinationRule

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: prod-rule
  namespace: myistio
spec:
  host: prodsvc
  trafficPolicy:
    loadBalancer:
      simple: ROUND_ROBIN
  subsets:
    - name: v1svc
      labels:
        app: prod
EOF
```

### 查看信息

```bash
➜ kubectl get Gateway -n myistio;
NAME            AGE
kiali-gateway   23h
prods-gateway   26m
➜ kubectl get DestinationRule -n myistio;
NAME        HOST      AGE
prod-rule   prodsvc   6m17s
➜ kubectl get VirtualService -n myistio;
NAME        GATEWAYS            HOSTS                  AGE
kialiinfo   ["kiali-gateway"]   ["kiali.inksnw.com"]   23h
prodvs      ["prods-gateway"]   ["prods.inksnw.com"]   14m
# 访问测试
curl http://prods.inksnw.com:32016/p/123
# 浏览器访问 http://kiali.inksnw.com:32016/
```

![iShot_2022-08-13_10.50.11](https://inksnw.asuscomm.com:3001/blog/iShot_2022-08-13_10.50.11-20220813105208779.png)

### 限流测试

修改dr文件,参考 https://istio.io/latest/docs/reference/config/networking/destination-rule/#ConnectionPoolSettings

```bash
➜ hey -c 2 -n 20 http://prods.inksnw.com:32016/p/123
Status code distribution:
  [200]	8 responses
  [503]	12 responses
```

### 融断测试

修改dr文件,参考 https://istio.io/latest/docs/reference/config/networking/destination-rule/#OutlierDetection

```yaml
outlierDetection:
	consecutive5xxErrors: 2
	interval: 10.0s
	maxEjectionPercent: 60
	baseEjectionTime: 10s
```

### 限制超时时间

修改ws文件,参考 https://istio.io/latest/docs/concepts/traffic-management/#timeouts

```yaml
timeout: 10s
```

