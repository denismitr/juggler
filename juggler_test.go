package juggler

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setTestNow() time.Time {
	return time.Now()
}

func TestCreateNewFile(t *testing.T) {
	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	currentTime = setTestNow
	now := setTestNow()

	j := New("test_log", dir)
	defer j.Close()

	b := []byte("test log")
	n, err := j.Write(b)
	assert.NoError(t, err)
	assert.Equal(t, len(b), n)
	expectedFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", now.Format(logFileSuffix)))
	assert.FileExists(t, expectedFile)

	ok, err := expectFileToContain(expectedFile, b)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestAppendToExistingFile(t *testing.T) {
	currentTime = setTestNow
	now := setTestNow()

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", now.Format(logFileSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := expectFileToContain(existingFile, entry)
	assert.NoError(t, err)
	assert.True(t, ok)

	j := New("test_log", dir)
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
	currentTime = setTestNow
	now := setTestNow()
	megabyte = 1

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", now.Format(logFileSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := expectFileToContain(existingFile, entry)
	assert.NoError(t, err)
	assert.True(t, ok)

	j := New("test_log", dir, WithMaxMegabytes(17))
	defer j.Close()

	nextFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.2.log", now.Format(logFileSuffix)))
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

	cleanUp, dir, err := createFakeLogFiles(
		"testDir",
		uf("2018-01-23", 1),
		uf("2018-01-25", 1),
		uf("2018-01-29", 1),
	)

	if err != nil {
		t.Fatal(err)
	}

	defer cleanUp()

	megabyte = 1

	currentTime = func() time.Time {
		t, err := time.Parse(logFileSuffix, "2018-01-29")
		if err != nil {
			panic(err)
		}
		return t
	}

	j := New(prefix, dir, WithCompression(), WithMaxMegabytes(17), WithNextTick(500 * time.Millisecond))
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

func TestRemoveTooManyBackups_NoVersions(t *testing.T) {
	prefix := "test_log"
	uf := uncompressedIdenticalTestFileFactory(prefix, "uncompressed fake - log - content")
	//cf := compressedIdenticalTestFileFactory(prefix, "compressed fake - log - content")

	cleanUp, dir, err := createFakeLogFiles(
		"testDir",
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

	currentTime = func() time.Time {
		t, err := time.Parse(logFileSuffix, "2018-01-30")
		if err != nil {
			panic(err)
		}
		return t
	}

	j := New(prefix, dir, WithMaxBackups(5), WithNextTick(500 * time.Millisecond))
	defer j.Close()

	logger := log.New(j, "foo", log.LstdFlags)
	logger.Println("bar")

	<-time.After(500 * time.Millisecond)

	shouldExist := []string{"2018-01-23", "2018-01-25", "2018-01-26", "2018-01-29", "2018-01-30"}
	shouldNotExist := []string{"2018-01-16", "2018-01-17", "2018-01-18", "2018-01-19", "2018-01-20", "2018-01-21", "2018-01-22"}

	for _, fn := range shouldExist {
		fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
		assert.FileExists(t, fp)
	}

	for _, fn := range shouldNotExist {
		fp := filepath.Join(dir, fmt.Sprintf("%s-%s.1.log", prefix, fn))
		assert.FileExists(t, fp)
	}
}