---
title: "本地docker作为镜像源"
date: 2024-08-30T15:48:13+08:00
tags: ["k8s"]
---

以前就有个想法, 一个节点拉取了镜像, 其它节点能不能直接以这个节点为源再拉取, 今天发现有一个[ spegel 项目](https://github.com/spegel-org/spegel)已经实现了 , 功能包括以存在的containerd为镜像源+p2p分发, 这里简单分析一个镜像源

### 最简单实现

这个需求最简单办法可以利用linux管道,直接转移

```bash
docker save busybox:latest | ssh root@192.168.50.131 "nerdctl -n k8s.io load"
```

### 镜像实现

把远程主机的的 `unix socket` 转发到本地

```
ssh -N -L /tmp/containerd.sock:/run/containerd/containerd.sock root@192.168.50.131
```

### 代码编写

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/errdefs"
	"github.com/gin-gonic/gin"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

var (
	Client      *containerd.Client
	ErrNotFound = errors.New("content not found")
)

var (
	nameRegex           = regexp.MustCompile(`([a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*)`)
	tagRegex            = regexp.MustCompile(`([a-zA-Z0-9_][a-zA-Z0-9._-]{0,127})`)
	manifestRegexTag    = regexp.MustCompile(`/v2/` + nameRegex.String() + `/manifests/` + tagRegex.String() + `$`)
	manifestRegexDigest = regexp.MustCompile(`/v2/` + nameRegex.String() + `/manifests/(.*)`)
	blobsRegexDigest    = regexp.MustCompile(`/v2/` + nameRegex.String() + `/blobs/(.*)`)
)

type reference struct {
	kind             string
	name             string
	dgst             digest.Digest
	originalRegistry string
}

func main() {
	r := gin.Default()
	r.GET("/v2/", welcomeHandler)
	r.GET("/v2/:ns/:name/manifests/:tag", handleManifest)
	r.HEAD("/v2/:ns/:name/manifests/:tag", handleManifest)
	r.GET("/v2/:ns/:name/blobs/:digest", handleBlob)
	r.Run(":5000")
}

func init() {
	addr := "/tmp/containerd.sock"
	var err error
	Client, err = containerd.New(addr, containerd.WithDefaultNamespace("k8s.io"))
	if err != nil {
		panic(err)
	}
}

func welcomeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Welcome to the Docker Registry API"})
}

func handleManifest(c *gin.Context) {
	ref, err := parsePathComponents(c.Request.URL.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if ref.dgst == "" {
		cImg, err := Client.GetImage(c.Request.Context(), ref.name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ref.dgst = cImg.Target().Digest
	}

	b, mediaType, err := GetManifest(c.Request.Context(), ref.dgst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", mediaType)
	c.Header("Content-Length", strconv.Itoa(len(b)))
	c.Header("Docker-Content-Digest", ref.dgst.String())

	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}

	c.Writer.Write(b)
}

func handleBlob(c *gin.Context) {
	ref, err := parsePathComponents(c.Request.URL.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	info, err := Client.ContentStore().Info(ctx, ref.dgst)
	if errors.Is(err, errdefs.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("blob with digest %s not found", ref.dgst.String())})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	c.Header("Docker-Content-Digest", ref.dgst.String())

	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}

	rc, err := GetBlob(ctx, ref.dgst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not get reader for blob with digest %s: %v", ref.dgst.String(), err)})
		return
	}
	defer rc.Close()

	io.Copy(c.Writer, rc)
}

func GetBlob(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error) {
	ra, err := Client.ContentStore().ReaderAt(ctx, ocispec.Descriptor{Digest: dgst})
	if errors.Is(err, errdefs.ErrNotFound) {
		return nil, errors.Join(ErrNotFound, err)
	}
	if err != nil {
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: content.NewReader(ra),
		Closer: ra,
	}, nil
}

func parsePathComponents(path string) (reference, error) {
	originalRegistry := "docker.io"
	if comps := manifestRegexTag.FindStringSubmatch(path); len(comps) == 6 {
		name := fmt.Sprintf("%s/%s:%s", originalRegistry, comps[1], comps[5])
		return reference{
			kind:             "Manifest",
			name:             name,
			originalRegistry: originalRegistry,
		}, nil
	}
	if comps := manifestRegexDigest.FindStringSubmatch(path); len(comps) == 6 {
		return reference{
			kind:             "Manifest",
			dgst:             digest.Digest(comps[5]),
			originalRegistry: originalRegistry,
		}, nil
	}
	if comps := blobsRegexDigest.FindStringSubmatch(path); len(comps) == 6 {
		return reference{
			kind:             "Blob",
			dgst:             digest.Digest(comps[5]),
			originalRegistry: originalRegistry,
		}, nil
	}
	return reference{}, errors.New("distribution path could not be parsed")
}

func GetManifest(ctx context.Context, dgst digest.Digest) ([]byte, string, error) {
	b, err := content.ReadBlob(ctx, Client.ContentStore(), ocispec.Descriptor{Digest: dgst})
	if errors.Is(err, errdefs.ErrNotFound) {
		return nil, "", errors.Join(ErrNotFound, err)
	}
	if err != nil {
		return nil, "", err
	}
	mt, err := DetermineMediaType(b)
	if err != nil {
		return nil, "", err
	}
	return b, mt, nil
}

func DetermineMediaType(b []byte) (string, error) {
	var ud struct {
		MediaType     string `json:"mediaType"`
		SchemaVersion int    `json:"schemaVersion"`
	}
	if err := json.Unmarshal(b, &ud); err != nil {
		return "", err
	}
	if ud.SchemaVersion == 2 && ud.MediaType != "" {
		return ud.MediaType, nil
	}
	data := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &data); err != nil {
		return "", err
	}
	_, architectureOk := data["architecture"]
	_, osOk := data["os"]
	_, rootfsOk := data["rootfs"]
	if architectureOk && osOk && rootfsOk {
		return ocispec.MediaTypeImageConfig, nil
	}
	_, manifestsOk := data["manifests"]
	if ud.SchemaVersion == 2 && manifestsOk {
		return ocispec.MediaTypeImageIndex, nil
	}
	_, configOk := data["config"]
	if ud.SchemaVersion == 2 && configOk {
		return ocispec.MediaTypeImageManifest, nil
	}
	return "", errors.New("not able to determine media type")
}
```

代码逻辑比较简单主要实现两个路由一个是查询 `manifest` 镜像层级配置清单,  一个是根据清单下载 `Blob` 资源

### 验证

shell 验证

```bash
curl http://127.0.0.1:5000/v2/mirrorgooglecontainers/defaultbackend-amd64/manifests/1.4 
```

响应返回 manifest 清单

```json
{
    "schemaVersion": 2,
    "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
    "config": {
        "mediaType": "application/vnd.docker.container.image.v1+json",
        "size": 1635,
        "digest": "sha256:846921f0fe0e57df9e4d4961c0c4af481bf545966b5f61af68e188837363530e"
    },
    "layers": [
        {
            "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
            "size": 1816013,
            "digest": "sha256:5f68dfd9f8d78f3535856ecc443b69b110c4003bd402e0e45996ef85b93cce79"
        }
    ]
}
```

在主机上查看,可以看到确实有这层资源

```bash
➜ ls -ltrh /var/lib/containerd/io.containerd.content.v1.content/blobs/sha256|grep 5f68dfd9f8d7
-r--r--r-- 1 root root 1.8M Jun 21 02:24 5f68dfd9f8d78f3535856ecc443b69b110c4003bd402e0e45996ef85b93cce79
```

手动拉取

```bash
curl  -o tt.tgz http://127.0.0.1:5000/v2/mirrorgooglecontainers/defaultbackend-amd64/blobs/sha256:5f68dfd9f8d78f3535856ecc443b69b110c4003bd402e0e45996ef85b93cce79
```

使用docker 拉取,配置镜像源

```json
{
    "registry-mirrors": [
        "http://127.0.0.1:5000"
    ]
}
```

拉取

```bash
d➜ Desktop docker pull mirrorgooglecontainers/defaultbackend-amd64:1.4                                 
1.4: Pulling from mirrorgooglecontainers/defaultbackend-amd64
5f68dfd9f8d7: Pull complete 
Digest: sha256:05cb942c5ff93ebb6c63d48737cd39d4fa1c6fa9dc7a4d53b2709f5b3c8333e8
Status: Downloaded newer image for mirrorgooglecontainers/defaultbackend-amd64:1.4
docker.io/mirrorgooglecontainers/defaultbackend-amd64:1.4
```

