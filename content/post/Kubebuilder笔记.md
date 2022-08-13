---
title: "Kubebuilder笔记"
date: 2022-08-09T23:47:44+08:00

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

