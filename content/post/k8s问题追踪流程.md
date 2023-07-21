---
title: "K8s问题追踪流程"
date: 2023-07-20T17:46:28+08:00
draft: true
---

```flow
st=>start: Start
op=>operation: Your Operation
cond=>condition: Yes or No?
e=>end

st->op->cond
cond(yes)->e
cond(no)->op
```
