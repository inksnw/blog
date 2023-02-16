---
title: "使用k8s进行leader选举"
date: 2023-02-16T13:59:00+08:00
tags: ["k8s"]
---

## 示例代码

```go
package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"log"
	"time"
)

func InitClient() *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatal(err)
	}
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	return c
}

func main() {
	id := "app2"
	client := InitClient()
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "myapp-lock",
			Namespace: "default",
		},
		Client:     client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{Identity: id},
	}

	leaderelection.RunOrDie(context.Background(), leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second, //租约时间  必须要大于RenewDeadline
		RenewDeadline:   20 * time.Second, // leader持有锁时间 在renewDeadline 内没有成功地更新，将释放锁,必须大于 RetryPeriod*1.2
		RetryPeriod:     5 * time.Second,  // 其它成员重试(竞争leader)时间间隔
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				for {
					fmt.Printf("业务运行中,当前实例是 %s\n", id)
					time.Sleep(time.Second * 3)
				}
			},
			OnStoppedLeading: func() {

			},
			OnNewLeader: func(identity string) {
				if identity == id {
					println("我还是leader,保持")
					return
				}
				fmt.Println("领导不是自己，是:", identity)
			},
		},
	})

}

```

## 查看租约

```bash
kubectl get Lease myapp-lock 
NAME         HOLDER   AGE
myapp-lock   app1     12m
```

