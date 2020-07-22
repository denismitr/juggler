package juggler

import (
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
)

type storage interface {
	start(runCh <-chan struct{}, errCh chan<- error)
}

type localCompression struct {
	dir    string
	prefix string
	format *regexp.Regexp
	tz     *time.Location
	mu sync.Mutex
	processing bool
}

func newLocalCompression(dir, prefix string, format *regexp.Regexp, tz *time.Location) *localCompression {
	return &localCompression{
		dir:    dir,
		prefix: prefix,
		format: format,
		tz:     tz,
	}
}

func (b *localCompression) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		b.mu.Lock()
		if b.processing {
			b.mu.Unlock()
			continue
		}
		b.processing = true
		b.mu.Unlock()

		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.tz)
		if err != nil {
			errCh <- err
			continue
		}

		nextCh := make(chan string, len(files))

		for _, f := range files {
			wg.Add(1)
			go compress(f.fullPath(), &wg, errCh, nil)
		}

		// do not wait for this one
		// todo: remove
		go func() {
			for f := range nextCh {
				fmt.Println(f)
			}
		}()

		wg.Wait()
		close(nextCh)
	}
}

type limitedStorage struct {
	mu sync.Mutex
	processing bool
	dir    string
	prefix string
	format *regexp.Regexp
	tz     *time.Location
	maxBackups int
}

func newLimitedStorage(maxBackups int, dir, prefix string, format *regexp.Regexp, tz *time.Location) *limitedStorage {
	return &limitedStorage{
		dir:    dir,
		prefix: prefix,
		format: format,
		tz:     tz,
		maxBackups: maxBackups,
	}
}

func (b *limitedStorage) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		b.mu.Lock()
		if b.processing {
			b.mu.Unlock()
			continue
		}

		b.processing = true

		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.tz)
		if err != nil {
			errCh <- err
			continue
		}

		var filesToDelete []logFileMeta
		if len(files) > b.maxBackups {
			filesToDelete = files[:len(files) - b.maxBackups]
		}

		for i := range filesToDelete {
			wg.Add(1)
			go func(f logFileMeta) {
				if err := os.Remove(f.fullPath()); err != nil {
					errCh <- err
				}

				wg.Done()
			}(files[i])
		}

		wg.Wait()

		b.processing = false
		b.mu.Unlock()
	}
}

