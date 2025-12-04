package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	endpoint   = "192.168.50.87:9000"
	bucketName = "blog"
	url        = "http://inksnw.asuscomm.com:3001"
)

type Info struct {
	AccessKeyID     string `json:"accessKey"`
	SecretAccessKey string `json:"secretKey"`
	ArticleName     string
	Images          []string
}

func main() {
	fmt.Printf("输入参数为 %s\n", os.Args)
	if len(os.Args) < 3 {
		fmt.Println("参数不足")
		return
	}

	config := getKey()
	awsConfig := getAWSConfig(config.AccessKeyID, config.SecretAccessKey)
	s, err := session.NewSession(&awsConfig)
	check(err)
	result := "Upload Success:\n"
	for _, filePath := range config.Images {
		fmt.Println("要上传的图片为: ", filePath)
		objectName := fmt.Sprintf("%s_%s%s", config.ArticleName, getMd5(filePath), path.Ext(filePath))
		body, err := os.ReadFile(filePath)
		check(err)
		rv := fmt.Sprintf("%s/%s/%s\n", url, bucketName, objectName)
		downloader := s3manager.NewDownloader(s)
		writer := aws.NewWriteAtBuffer([]byte{})
		_, err = downloader.Download(writer, &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
		})
		result = result + rv
		if err == nil {
			fmt.Printf("文件 %s 已存在\n", objectName)
			continue
		}
		uploadFile(s, bucketName, objectName, body)
	}
	fmt.Print(result)
}
func getAWSConfig(accessKeyID, secretAccessKey string) aws.Config {
	cred := credentials.NewStaticCredentials(accessKeyID, secretAccessKey, "")
	return aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String(endpoint),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      cred,
	}
}

func uploadFile(s *session.Session, bucketName, objectName string, body []byte) {
	uploader := s3manager.NewUploader(s, func(uploader *s3manager.Uploader) {
		uploader.LeavePartsOnError = true
	})
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   bytes.NewReader(body),
	})
	check(err)
}

func getKey() (result Info) {
	u, err := user.Current()
	check(err)
	file := path.Join(u.HomeDir, "/Desktop", "/blog", "env.json")
	data, err := os.ReadFile(file)
	check(err)
	err = json.Unmarshal(data, &result)
	check(err)
	result.ArticleName = os.Args[1]
	result.Images = os.Args[2:]
	return result
}

func getMd5(filename string) string {
	pFile, err := os.Open(filename)
	check(err)
	md5h := md5.New()
	_, err = io.Copy(md5h, pFile)
	check(err)
	defer pFile.Close()
	return hex.EncodeToString(md5h.Sum(nil))
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
