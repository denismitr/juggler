# Juggler

## Daily log rotator with S3 support

## Usage

### With compression and upload to S3
```go
cloudUploader, err := cloud.New(cloud.Config{
		Id: "minio",
		Secret: "minio123",
		Bucket: "testbucket",
		Endpoint: "http://127.0.0.1:9001",
		Acl: "public-read",
		Region: "us-east-1",
		NoSSL: true,
	})

	if err != nil {
		panic(err)
	}

	errCh := make(chan error)
    go func() {
        for err := range errCh {
            t.Fatal(err)
        }
    }()

	j := New(
		"my-log-file",
		"/var/log/mylogs/",
		WithCompressionAndCloudUploader(cloudUploader),
		WithMaxMegabytes(25),
	)

	defer j.Close()

	j.NotifyOnError(errCh)

    logger := log.New(j, "foo", log.LstdFlags)
    logger.Println("bar")
```

```
/var/log/mylogs/my-log-file-2020-10-11.1.log
/var/log/mylogs/my-log-file-2020-10-11.1.log.gz // next day will be compressed and uploaded to S3
```

### Tests
```make minio```
```make test```