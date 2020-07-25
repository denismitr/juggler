package juggler

import (
	"fmt"
	"github.com/denismitr/juggler/cloud"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateNewFile(t *testing.T) {
	dir := makeTestDir(randomString(20), t)
	defer os.RemoveAll(dir)

	nowFunc := createNowFunc(dateSuffix, "2020-01-01")
	now := nowFunc()

	j := New("test_log", dir, withNowFunc(nowFunc))
	defer j.Close()

	b := []byte("test log")
	n, err := j.Write(b)
	assert.NoError(t, err)
	assert.Equal(t, len(b), n)
	expectedFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", now.Format(dateSuffix)))
	assert.FileExists(t, expectedFile)

	ok, err := expectFileToContain(expectedFile, b)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestAppendToExistingFile(t *testing.T) {
	nowFunc := createNowFunc(dateSuffix, "2020-01-01")
	now := nowFunc()

	dir := makeTestDir(randomString(15), t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", now.Format(dateSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := expectFileToContain(existingFile, entry)
	assert.NoError(t, err)
	assert.True(t, ok)

	j := New("test_log", dir, withNowFunc(nowFunc))
	defer j.Close()

	nextEntry := []byte("nextEntry\n")
	n, err := j.Write(nextEntry)

	assert.NoError(t, err)
	assert.Equal(t, n, len(nextEntry))

	ok, err = expectFileToContain(existingFile, append(entry, nextEntry...))
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestJugglingDuringWrite(t *testing.T) {
	nowFunc := createNowFunc(dateSuffix, "2018-01-30")
	megabyte = 1

	dir := makeTestDir(randomString(15), t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", nowFunc().Format(dateSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := expectFileToContain(existingFile, entry)
	assert.NoError(t, err)
	assert.True(t, ok)

	j := New("test_log", dir, WithMaxMegabytes(17), withNowFunc(nowFunc))
	defer j.Close()

	nextFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.2.log", nowFunc().Format(dateSuffix)))
	nextEntry := []byte("next log too big")
	n, err := j.Write(nextEntry)
	assert.NoError(t, err)
	assert.Equal(t, len(nextEntry), n)

	ok, err = expectFileToContain(nextFile, nextEntry)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestCompressAfterJuggle(t *testing.T) {
	prefix := "test_log"
	content := "uncompressed fake - log - content"
	uf := uncompressedIdenticalTestFileFactory(prefix, content)

	nowFunc := createNowFunc(dateSuffix, "2018-01-29")

	cleanUp, dir, err := createFakeLogFiles(
		randomString(15),
		uf("2018-01-23", 1),
		uf("2018-01-25", 1),
		uf("2018-01-29", 1),
	)

	if err != nil {
		t.Fatal(err)
	}

	defer cleanUp()

	megabyte = 1

	j := New(
		prefix,
		dir,
		WithCompression(),
		WithMaxMegabytes(17),
		WithNextTick(500 * time.Millisecond),
		withNowFunc(nowFunc),
	)

	defer j.Close()

	prevFile := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, "2018-01-29"))
	nextFile := filepath.Join(dir, fmt.Sprintf("%s-%s.2.log", prefix, "2018-01-29"))
	nextEntry := []byte("next log too big")
	n, err := j.Write(nextEntry)

	assert.NoError(t, err)
	assert.Equal(t, len(nextEntry), n)

	ok, err := expectFileToContain(nextFile, nextEntry)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = expectFileToContain(prevFile, []byte(content))
	assert.NoError(t, err)
	assert.True(t, ok)

	<-time.After(1 * time.Second)

	assert.FileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)

	prevFile = filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, "2018-01-25"))
	assert.FileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)

	prevFile = filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, "2018-01-23"))
	assert.FileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)
}

func TestCompressAndUploadAfterJuggle(t *testing.T) {
	prefix := "test_log"
	content := "uncompressed fake - log - content"
	uf := uncompressedIdenticalTestFileFactory(prefix, content)

	nowFunc := createNowFunc(dateSuffix, "2018-01-29")

	cleanUp, dir, err := createFakeLogFiles(
		randomString(13),
		uf("2018-01-23", 1),
		uf("2018-01-25", 1),
		uf("2018-01-29", 1),
	)

	if err != nil {
		t.Fatal(err)
	}

	megabyte = 1

	uploader, err := cloud.New(cloud.Config{
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

	j := New(
		prefix,
		dir,
		WithCompressionAndCloudUploader(uploader),
		WithMaxMegabytes(17),
		WithNextTick(500 * time.Millisecond),
		withNowFunc(nowFunc),
	)

	j.NotifyOnError(errCh)

	go func() {
		for err := range errCh {
			panic(err)
		}
	}()

	prevFile := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, "2018-01-29"))
	nextFile := filepath.Join(dir, fmt.Sprintf("%s-%s.2.log", prefix, "2018-01-29"))
	nextEntry := []byte("next log too big")
	n, err := j.Write(nextEntry)

	assert.NoError(t, err)
	assert.Equal(t, len(nextEntry), n)

	ok, err := expectFileToContain(nextFile, nextEntry)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = expectFileToContain(prevFile, []byte(content))
	assert.NoError(t, err)
	assert.True(t, ok)

	<-time.After(1 * time.Second)

	assert.NoFileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)

	prevFile = filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, "2018-01-25"))
	assert.NoFileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)

	prevFile = filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, "2018-01-23"))
	assert.NoFileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)

	j.Close()
	<-time.After(2000 * time.Millisecond)
	cleanUp()
}

func TestRemoveTooManyBackups(t *testing.T) {
	prefix := "test_log"
	uf := uncompressedIdenticalTestFileFactory(prefix, "uncompressed fake - log - content")
	cf := compressedIdenticalTestFileFactory(prefix, "compressed fake - log - content")

	nowFunc := createNowFunc(dateSuffix, "2018-01-30")

	t.Run("no versions and not compressed files", func(t *testing.T) {
		cleanUp, dir, err := createFakeLogFiles(
			randomString(15),
			uf("2018-01-16", 1),
			uf("2018-01-17", 1),
			uf("2018-01-18", 1),
			uf("2018-01-19", 1),
			uf("2018-01-20", 1),
			uf("2018-01-21", 1),
			uf("2018-01-22", 1),
			uf("2018-01-23", 1),
			uf("2018-01-25", 1),
			uf("2018-01-26", 1),
			uf("2018-01-29", 1),
		)

		if err != nil {
			t.Fatal(err)
		}

		defer cleanUp()

		j := New(prefix, dir, WithMaxBackups(5), WithNextTick(250 * time.Millisecond), withNowFunc(nowFunc))
		defer j.Close()

		logger := log.New(j, "foo", log.LstdFlags)
		logger.Println("bar")

		<-time.After(800 * time.Millisecond)

		shouldExist := []string{"2018-01-22", "2018-01-23", "2018-01-25", "2018-01-26", "2018-01-29", "2018-01-30"} // 30 is today file, so it does not count
		shouldNotExist := []string{"2018-01-16", "2018-01-17", "2018-01-18", "2018-01-19", "2018-01-20", "2018-01-21"}

		for _, fn := range shouldExist {
			fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
			assert.FileExists(t, fp)
		}

		for _, fn := range shouldNotExist {
			fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
			assert.NoFileExists(t, fp)
		}
	})

	t.Run("no versions and compressed files should be left untouched", func(t *testing.T) {
		cleanUp, dir, err := createFakeLogFiles(
			randomString(14),
			uf("2018-01-16", 1),
			cf("2018-01-17", 1),
			uf("2018-01-18", 1),
			cf("2018-01-19", 1),
			cf("2018-01-20", 1),
			uf("2018-01-21", 1),
			uf("2018-01-22", 1),
			uf("2018-01-23", 1),
			uf("2018-01-25", 1),
			uf("2018-01-26", 1),
			uf("2018-01-29", 1),
		)

		if err != nil {
			t.Fatal(err)
		}

		defer cleanUp()

		j := New(prefix, dir, WithMaxBackups(5), WithNextTick(250 * time.Millisecond), withNowFunc(nowFunc))
		defer j.Close()

		<-time.After(800 * time.Millisecond)

		shouldExist := []string{"2018-01-22", "2018-01-23", "2018-01-25", "2018-01-26", "2018-01-29"}
		compressedShouldExist := []string{"2018-01-17", "2018-01-19", "2018-01-20"}
		shouldNotExist := []string{"2018-01-16", "2018-01-18", "2018-01-21"}

		for _, fn := range shouldExist {
			fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
			assert.FileExists(t, fp)
		}

		for _, fn := range shouldNotExist {
			fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
			assert.NoFileExists(t, fp)
		}

		for _, fn := range compressedShouldExist {
			fp := filepath.Join(dir, gzippedName(fmt.Sprintf("%s-%s.1.log", prefix, fn)))
			assert.FileExists(t, fp)
		}
	})

	t.Run("today file is not counted", func(t *testing.T) {
		cleanUp, dir, err := createFakeLogFiles(
			randomString(14),
			uf("2018-01-22", 1),
			uf("2018-01-23", 1),
			uf("2018-01-25", 1),
			uf("2018-01-26", 1),
			uf("2018-01-29", 1),
		)

		if err != nil {
			t.Fatal(err)
		}

		defer cleanUp()

		j := New(prefix, dir, WithMaxBackups(5), WithNextTick(250 * time.Millisecond), withNowFunc(nowFunc))
		defer j.Close()

		logger := log.New(j, "foo", log.LstdFlags)
		logger.Println("bar")

		<-time.After(800 * time.Millisecond)

		// 30 is today file, so it does not count
		shouldExist := []string{"2018-01-22", "2018-01-23", "2018-01-25", "2018-01-26", "2018-01-29", "2018-01-30"}

		for _, fn := range shouldExist {
			fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
			assert.FileExists(t, fp)
		}
	})
}