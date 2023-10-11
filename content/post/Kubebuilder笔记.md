---
title: "Kubebuilder笔记"
date: 2022-08-09T23:47:44+08:00
tags: ["k8s"]
---

记录一下方便自己使用

```bash
mkdir myoperator
cd myoperator
go mod init myoperator
kubebuilder init --domain my.domain
kubebuilder create api --group webapp --version v1 --kind Guestbook
#Create Resource [y/n]
#y
#Create Controller [y/n]
#y
make generate
```

一段代码笔记

```go
package main

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

type GuestbookReconciler struct{}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		panic(err)
	}
	if err = (&GuestbookReconciler{}).SetupWithManager(mgr); err != nil {
		panic(err)
	}
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

func (r *GuestbookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	now := time.Now()
	fmt.Printf("%s: %s reconcile\n", now, req.Name)
	//出错立即重入
	//return ctrl.Result{}, errors.New("test")
	// 手动重入
	//return ctrl.Result{Requeue: true}, nil
	// 定时重入
	return ctrl.Result{RequeueAfter: time.Second * 3}, nil
}

func (r *GuestbookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
```

