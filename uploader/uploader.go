package uploader

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"os"
)

type Config struct {
	region string
	id     string
	secret string
	bucket string
	folder string
	acl    string
}

type S3GzipUploader struct {
	cfg Config
	s   *session.Session
}

func New(cfg Config) (*S3GzipUploader, error) {
	if cfg.bucket == "" {
		return nil, errors.Errorf("Bucket is required")
	}

	uploader := &S3GzipUploader{cfg: cfg}
	if err := uploader.connect(); err != nil {
		return nil, err
	}

	return uploader, nil
}

func (u *S3GzipUploader) connect() error {
	s, err := session.NewSession(&aws.Config{
		Region:      aws.String(u.cfg.region),
		Credentials: credentials.NewStaticCredentials(u.cfg.id, u.cfg.secret, ""),
	})

	if err != nil {
		return err
	}

	u.s = s

	return nil
}

func (u *S3GzipUploader) Upload(filepath string) error {
	if u.s == nil {
		if err := u.connect(); err != nil {
			return err
		}
	}

	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	size := stat.Size()
	input := &s3.PutObjectInput{
		Body:            f,
		Bucket:          aws.String(u.cfg.bucket),
		Key:             aws.String(u.resolveKeyFor(filepath)),
		ContentType:     aws.String("text/plain"),
		ContentLength:   aws.Int64(size),
		ContentEncoding: aws.String("gzip"),
	}

	if u.cfg.acl != "" {
		input.ACL = aws.String(u.cfg.acl)
	}

	_, err = s3.New(u.s).PutObject(input)

	if err != nil {
		return err
	}

	return nil
}

func (u *S3GzipUploader) resolveKeyFor(filepath string) string {
	return fmt.Sprintf("%s/%s", u.cfg.folder, filepath)
}
