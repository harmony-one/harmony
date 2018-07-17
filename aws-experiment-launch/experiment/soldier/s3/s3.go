package s3

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func createSession() *session.Session {
	return session.Must(session.NewSession(&aws.Config{
		Region:     aws.String(endpoints.UsWest2RegionID),
		MaxRetries: aws.Int(3),
	}))
}

func CreateBucket(bucketName string, region string) {
	sess := createSession()
	svc := s3.New(sess)
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(region),
		},
	}

	result, err := svc.CreateBucket(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				fmt.Println(s3.ErrCodeBucketAlreadyExists, aerr.Error())
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				fmt.Println(s3.ErrCodeBucketAlreadyOwnedByYou, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}

	fmt.Println(result)
}

func UploadFile(bucketName string, fileName string, key string) (result *s3manager.UploadOutput, err error) {
	sess := createSession()
	uploader := s3manager.NewUploader(sess)

	f, err := os.Open(fileName)
	if err != nil {
		log.Println("Failed to open file", err)
		return nil, err
	}
	// Upload the file to S3.
	result, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		log.Println("failed to upload file", err)
		return nil, err
	}
	fmt.Printf("file uploaded to, %s\n", result.Location)
	return result, nil
}

func DownloadFile(bucketName string, fileName string, key string) (n int64, err error) {
	sess := createSession()

	downloader := s3manager.NewDownloader(sess)

	f, err := os.Create(fileName)
	if err != nil {
		return
	}

	n, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	return
}
