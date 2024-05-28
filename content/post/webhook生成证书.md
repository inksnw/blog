---
title: "Webhook生成证书"
date: 2024-05-28T12:48:47+08:00
tags: ["k8s"]
---

## 创建一个webhook服务

```go
package main

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	entryLog := log.Log.WithName("entrypoint")
	entryLog.Info("setting up manager")
	webhookServer := webhook.NewServer(webhook.Options{
		CertDir: "/Users/inksnw/Desktop/builtins/cert",
		Port:    443,
	})
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		WebhookServer: webhookServer,
	})
	if err != nil {
		entryLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	if err = builder.WebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithDefaulter(&podAnnotator{}).
		WithValidator(&podValidator{}).
		Complete(); err != nil {
		entryLog.Error(err, "unable to create webhook", "webhook", "Pod")
		os.Exit(1)
	}

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io

// podAnnotator annotates Pods
type podAnnotator struct{}

func (a *podAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected a Pod but got a %T", obj)
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations["example-mutating-admission-webhook"] = "foo"
	log.Info("Annotated Pod")

	return nil
}

// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=vpod.kb.io

// podValidator validates Pods
type podValidator struct{}

// validate admits a pod if a specific annotation exists.
func (v *podValidator) validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	log := logf.FromContext(ctx)
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected a Pod but got a %T", obj)
	}

	log.Info("Validating Pod")
	key := "example-mutating-admission-webhook"
	anno, found := pod.Annotations[key]
	if !found {
		return nil, fmt.Errorf("missing annotation %s", key)
	}
	if anno != "foo" {
		return nil, fmt.Errorf("annotation %s did not have value %q", key, "foo")
	}

	return nil, nil
}

func (v *podValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}

func (v *podValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, newObj)
}

func (v *podValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}

```

## 生成证书csr请求

```bash
[ req ]
default_bits       = 2048
prompt             = no
default_md         = sha256
distinguished_name = dn
req_extensions     = req_ext

[ dn ]
CN = system:node:webhook-service.default.svc
O = system:nodes

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = webhook-service.default.svc
```

csr.conf

```bash
#!/bin/bash

openssl genrsa -out tls.key 2048
openssl req -new -key tls.key -config csr.conf -out webhook-service.csr

cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: webhook-service.csr
spec:
  request: $(cat webhook-service.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kubelet-serving
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

kubectl certificate approve webhook-service.csr
kubectl get csr webhook-service.csr -o jsonpath='{.status.certificate}' | base64 --decode > tls.crt
```

生成证书文件

```bash
bash csr.sh
tree
.
├── csr.conf
├── csr.sh
├── install.yaml
├── tls.crt
├── tls.key
└── webhook-service.csr

1 directory, 6 files
```

## 配置 Webhook

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: example-mutating-webhook
webhooks:
  - name: example-webhook.yourdomain.com
    clientConfig:
      caBundle: xxx # tls.crt转base64
      service:
        name: webhook-service
        namespace: default
        path: "/mutate--v1-pod"
    rules:
      - operations: [ "CREATE", "UPDATE" ]
        apiGroups: [ "" ]
        apiVersions: [ "v1" ]
        resources: [ "pods" ]
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1", "v1beta1" ]
    timeoutSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: webhook-service
spec:
  type: ExternalName
  externalName: nuc.inksnw.com
```

## 验证

```bash
kubectl run demo --image=nginx
# 查看日志
{"level":"info","ts":"2024-05-28T12:48:02+08:00","logger":"admission","msg":"Annotated Pod","webhookGroup":"","webhookKind":"Pod","Pod":{"name":"demo","namespace":"default"},"namespace":"default","name":"demo","resource":{"group":"","version":"v1","resource":"pods"},"user":"kubernetes-admin","requestID":"c1929fd7-c2d0-4474-a677-d9ad98392cf0"}
```

