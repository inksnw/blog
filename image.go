package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"os"
	"os/user"
	"path"
)

const (
	endpoint   = "192.168.50.209:9000"
	bucketName = "blog"
	url        = "http://inksnw.asuscomm.com:3001"
)

func main() {
	if len(os.Args) < 3 {
		return
	}
	fmt.Printf("输入参数为 %s\n", os.Args)
	config := getKey()
	ctx := context.Background()
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
	})
	check(err)

	result := "Upload Success:\n"
	for _, filePath := range config.images {
		fmt.Println("要上传的图片为: ", filePath)
		objectName := fmt.Sprintf("%s_%s%s", config.articleName, getMd5(filePath), path.Ext(filePath))
		contentType := getExt(filePath)
		_, err := minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
		check(err)
		rv := fmt.Sprintf("%s/%s/%s\n", url, bucketName, objectName)
		result = result + rv
	}
	fmt.Print(result)
}

func getExt(file string) (contentType string) {
	ext := path.Ext(file)
	contentType = fmt.Sprintf("application/%s", ext[1:])
	return contentType
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Info struct {
	AccessKeyID     string `json:"accessKey"`
	SecretAccessKey string `json:"secretKey"`
	articleName     string
	images          []string
}

func getKey() (result Info) {
	u, err := user.Current()
	check(err)
	file := path.Join(u.HomeDir, "/Desktop", "/blog", "env.json")
	data, err := os.ReadFile(file)
	fmt.Printf("key文件为 %s\n", file)
	check(err)
	err = json.Unmarshal(data, &result)
	check(err)
	result.articleName = os.Args[1]
	result.images = os.Args[2:]
	return result
}
func getMd5(filename string) string {
	pFile, err := os.Open(filename)
	check(err)
	md5h := md5.New()
	_, err = io.Copy(md5h, pFile)
	check(err)
	return hex.EncodeToString(md5h.Sum(nil))
}
