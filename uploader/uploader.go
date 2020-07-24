package uploader

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"log"
	"os"
)

type Config struct {
	Region string
	Id     string
	Secret string
	Bucket string
	Acl    string
}

type S3GzipUploader struct {
	cfg Config
	s   *session.Session
}

func New(cfg Config) (*S3GzipUploader, error) {
	if cfg.Bucket == "" {
		return nil, errors.Errorf("Bucket is required")
	}

	u := &S3GzipUploader{cfg: cfg}
	if err := u.connect(); err != nil {
		return nil, err
	}

	return u, nil
}

func (u *S3GzipUploader) connect() error {
	ll := aws.LogDebugWithHTTPBody | aws.LogDebugWithSigning
	s, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String("http://127.0.0.1:9001"),
		DisableSSL:  aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials: credentials.NewStaticCredentials(u.cfg.Id, u.cfg.Secret, ""),
		LogLevel: &ll,
	})

	if err != nil {
		return errors.Wrapf(err, "could not connect to %s", "http://127.0.0.1:9001")
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

	// Create an uploader with the session and default options
	up := s3manager.NewUploader(u.s)
	res, err := up.Upload(&s3manager.UploadInput{
		Bucket:          aws.String("testbucket"),
		Key:             aws.String("testfile.gz"),
		Body:            f,
		ContentType:     aws.String("text/plain"),
		ContentEncoding: aws.String("gzip"),
		ACL:             aws.String("public-read"),
	})

	if err != nil {
		return errors.Wrapf(err, "could not put object %s to S3", "testfile.gz")
	}

	log.Println(res.Location)

	return nil
}
