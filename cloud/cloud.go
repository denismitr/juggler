package cloud

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

const logFileContentType = "text/plain"
const logFileContentEncoding = "gzip"

type Config struct {
	Region   string
	Id       string
	Secret   string
	Bucket   string
	Endpoint string
	Acl      string
	NoSSL    bool
}

type S3GzipCloud struct {
	cfg Config
	s   *session.Session
}

func New(cfg Config) (*S3GzipCloud, error) {
	if cfg.Bucket == "" {
		return nil, errors.Errorf("Bucket is required")
	}

	u := &S3GzipCloud{cfg: cfg}
	if err := u.connect(); err != nil {
		return nil, err
	}

	return u, nil
}

func (u *S3GzipCloud) connect() error {
	cfg := aws.Config{
		Region:           aws.String(u.cfg.Region),
		Endpoint:         aws.String(u.cfg.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(u.cfg.Id, u.cfg.Secret, ""),
		//LogLevel:         &ll,
	}

	if u.cfg.NoSSL {
		cfg.DisableSSL = aws.Bool(true)
	}

	//ll := aws.LogDebugWithHTTPBody | aws.LogDebugWithSigning
	s, err := session.NewSession(&cfg)

	if err != nil {
		return errors.Wrapf(err, "could not connect to %s", "http://127.0.0.1:9001")
	}

	u.s = s

	return nil
}

func (u *S3GzipCloud) Upload(fp string) error {
	if u.s == nil {
		if err := u.connect(); err != nil {
			return err
		}
	}

	f, err := os.Open(fp)
	if err != nil {
		return errors.Wrapf(err, "could not open %s to upload to the cloud", fp)
	}

	// Create an uploader with the session and default options
	up := s3manager.NewUploader(u.s)

	_, err = up.Upload(&s3manager.UploadInput{
		Bucket:          aws.String(u.cfg.Bucket),
		Key:             aws.String(filepath.Base(f.Name())),
		Body:            f,
		ContentType:     aws.String(logFileContentType),
		ContentEncoding: aws.String(logFileContentEncoding),
		ACL:             aws.String(u.cfg.Acl),
	})

	if err != nil {
		return errors.Wrapf(err, "could not put object %s to S3", "testfile.gz")
	}

	return nil
}
