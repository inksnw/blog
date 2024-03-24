package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"io"
	"os"
	"os/user"
	"path"
)

const (
	endpoint   = "192.168.50.87:9000"
	bucketName = "blog"
	url        = "http://inksnw.asuscomm.com:3001"
)

func main() {
	if len(os.Args) < 3 {
		return
	}
	fmt.Printf("输入参数为 %s\n", os.Args)
	config := getKey()
	cred := credentials.NewStaticCredentials(config.AccessKeyID, config.SecretAccessKey, "")
	awsConfig := aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String(endpoint),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      cred,
	}
	s, err := session.NewSession(&awsConfig)
	check(err)

	result := "Upload Success:\n"
	for _, filePath := range config.images {
		fmt.Println("要上传的图片为: ", filePath)
		objectName := fmt.Sprintf("%s_%s%s", config.articleName, getMd5(filePath), path.Ext(filePath))
		body, err := os.ReadFile(filePath)
		check(err)
		uploader := s3manager.NewUploader(s, func(uploader *s3manager.Uploader) {
			uploader.LeavePartsOnError = true
		})
		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
			Body:   bytes.NewReader(body),
		})

		check(err)
		rv := fmt.Sprintf("%s/%s/%s\n", url, bucketName, objectName)
		result = result + rv
	}
	fmt.Print(result)
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
