---
title: "Istio与jaeger"
date: 2022-08-17T22:37:31+08:00
tags: ["k8s"]
subs: ["istio"]
---

## 暴露出istio自带的jaeger

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: tracing-gateway
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http-tracing
        protocol: HTTP
      hosts:
        - tracing.inksnw.com
---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: tracing-vs
  namespace: istio-system
spec:
  hosts:
    - tracing.inksnw.com
  gateways:
    - tracing-gateway
  http:
    - route:
        - destination:
            host: tracing
            port:
              number: 80
---
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: tracing
  namespace: istio-system
spec:
  host: tracing
  trafficPolicy:
    tls:
      mode: DISABLE
```

## 访问

```bash
curl http://tracing.inksnw.com:32016/jaeger/search
```

