package juggler

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
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
	expectedFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
	assert.FileExists(t, expectedFile)
	expectFileToContain(t, expectedFile, b)
}

func TestAppendToExistingFile(t *testing.T) {
	currentTime = setTestNow
	now := setTestNow()

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	expectFileToContain(t, existingFile, entry)

	j := New("test_log", dir)
	defer j.Close()

	nextEntry := []byte("nextEntry\n")
	n, err := j.Write(nextEntry)

	assert.NoError(t, err)
	assert.Equal(t, n, len(nextEntry))
	expectFileToContain(t, existingFile, append(entry, nextEntry...))
}

func TestJugglingDuringWrite(t *testing.T) {
	currentTime = setTestNow
	now := setTestNow()
	mb = 1

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	expectFileToContain(t, existingFile, entry)

	j := New("test_log", dir, WithMaxMegabytes(17))
	defer j.Close()

	nextFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.2.log", now.Format(logFileSuffix)))
	nextEntry := []byte("next log too big")
	n, err := j.Write(nextEntry)
	assert.NoError(t, err)
	assert.Equal(t, len(nextEntry), n)
	expectFileToContain(t, nextFile, nextEntry)
}

func TestCompressAfterJuggle(t *testing.T) {
	prefix := "test_log"
	content := "uncompressed fake - log - content"
	uf := uncompressedIdenticalTestFileFactory(prefix, content)

	cleanUp, dir, err := createFakeLogFiles(
		"testDir",
		uf("2018-01-23", 0),
		uf("2018-01-25", 0),
		uf("2018-01-29", 0),
	)

	if err != nil {
		t.Fatal(err)
	}

	defer cleanUp()

	mb = 1

	currentTime = func() time.Time {
		t, err := time.Parse(logFileSuffix, "2018-01-29")
		if err != nil {
			panic(err)
		}
		return t
	}

	j := New(prefix, dir, WithMaxMegabytes(17), WithNextTick(500 * time.Millisecond))
	defer j.Close()

	prevFile := filepath.Join(dir, fmt.Sprintf("%s-%s.log", prefix, "2018-01-25"))
	nextFile := filepath.Join(dir, fmt.Sprintf("%s-%s.2.log", prefix, "2018-01-29"))
	nextEntry := []byte("next log too big")
	n, err := j.Write(nextEntry)

	assert.NoError(t, err)
	assert.Equal(t, len(nextEntry), n)
	expectFileToContain(t, nextFile, nextEntry)
	expectFileToContain(t, prevFile, []byte(content))

	<-time.After(1 * time.Second)

	assert.FileExists(t, gzippedName(prevFile))
	assert.NoFileExists(t, prevFile)
}
