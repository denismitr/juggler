package juggler

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type testLogFile struct {
	date       string
	prefix     string
	version    int
	compressed bool
	content    string
}

func location(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}

	return l
}

const testTimeFormat = "2006-01-02T15-04-05.000"

func uncompressedTestFileFactory(prefix string) func(string, string, int) testLogFile {
	return func(date string, content string, version int) testLogFile {
		return testLogFile{prefix: prefix, content: content, compressed: false, date: date, version: version}
	}
}

func uncompressedIdenticalTestFileFactory(prefix, content string) func(date string, version int) testLogFile {
	return func(date string, version int) testLogFile {
		return testLogFile{prefix: prefix, content: content, compressed: false, date: date, version: version}
	}
}

func compressedIdenticalTestFileFactory(prefix, content string) func(date string, version int) testLogFile {
	return func(date string, version int) testLogFile {
		return testLogFile{prefix: prefix, content: content, compressed: true, date: date, version: version}
	}
}

func createFakeLogFiles(dirName string, tfs ...testLogFile) (func(), string, error) {
	dir, err := createTestDir(dirName)
	if err != nil {
		return nil, "", err
	}

	var f string

	for _, tf := range tfs {
		if tf.version == 0 {
			f = filepath.Join(dir, fmt.Sprintf("%s-%s.log", tf.prefix, tf.date))
		} else {
			f = filepath.Join(dir, fmt.Sprintf("%s-%s.%d.log", tf.prefix, tf.date, tf.version))
		}

		if tf.compressed {
			f += ".gz"
		}

		entry := []byte(tf.content)

		err := ioutil.WriteFile(f, entry, 0644)
		if err != nil {
			return nil, "", err
		}
	}

	return func() {
		_ = os.RemoveAll(dir)
	}, dir, nil
}

func createTestDir(name string) (string, error) {
	dir := filepath.Join(os.TempDir(), name)
	if err := os.Mkdir(dir, 0700); err != nil {
		if !os.IsExist(err) {
			return "", err
		}
	}

	return dir, nil
}
