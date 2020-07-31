package juggler

import (
	"github.com/pkg/errors"
	"os"
	"regexp"
	"sync"
	"time"
)

type storage interface {
	start(runCh <-chan struct{}, errCh chan<- error)
}

type uploader interface {
	Upload(filepath string) error
}

func (j *Juggler) createStorage() storage {
	if j.uploader != nil && j.compression {
		return newCloudCompression(j.directory, j.prefix, j.uploader, j.format, j.nowFunc, j.timezone)
	}

	if j.compression {
		return newLocalCompression(j.directory, j.prefix, j.format, j.nowFunc, j.timezone)
	}

	return newLimitedStorage(j.maxBackups, j.directory, j.prefix, j.format, j.nowFunc, j.timezone)
}

type base struct {
	dir    string
	prefix string
	format *regexp.Regexp
	tz     *time.Location
	nowFunc func() time.Time
}

type localCompression struct {
	base
}

func newLocalCompression(dir, prefix string, format *regexp.Regexp, nowFunc nowFunc, tz *time.Location) *localCompression {
	return &localCompression{
		base: base{
			dir:    dir,
			prefix: prefix,
			format: format,
			tz:     tz,
			nowFunc: nowFunc,
		},
	}
}

func (b *localCompression) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.nowFunc, b.tz)
		if err != nil {
			errCh <- err
			continue
		}

		for _, f := range files {
			wg.Add(1)
			go compressAndRemove(f.fullPath(), &wg, errCh, nil)
		}

		wg.Wait()
	}
}

type limitedStorage struct {
	base
	maxBackups int
}

func newLimitedStorage(maxBackups int, dir, prefix string, format *regexp.Regexp, nowFunc nowFunc, tz *time.Location) *limitedStorage {
	return &limitedStorage{
		base: base{
			dir:    dir,
			prefix: prefix,
			format: format,
			tz:     tz,
			nowFunc: nowFunc,
		},
		maxBackups: maxBackups,
	}
}

func (b *limitedStorage) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.nowFunc, b.tz)
		if err != nil {
			errCh <- err
			continue
		}

		var filesToDelete []logFileMeta
		if len(files) > b.maxBackups {
			filesToDelete = files[:len(files)-b.maxBackups]
		}

		for i := range filesToDelete {
			wg.Add(1)
			go func(f logFileMeta) {
				if err := os.Remove(f.fullPath()); err != nil {
					errCh <- errors.Wrapf(err, "could not delete %s", f.fullPath())
				}

				wg.Done()
			}(files[i])
		}

		wg.Wait()
	}
}

type cloudCompression struct {
	base
	uploader uploader
}

func newCloudCompression(dir, prefix string, uploader uploader, format *regexp.Regexp, nowFunc nowFunc, tz *time.Location) *cloudCompression {
	return &cloudCompression{
		base: base{
			dir:    dir,
			prefix: prefix,
			format: format,
			tz:     tz,
			nowFunc: nowFunc,
		},
		uploader: uploader,
	}
}

func (b *cloudCompression) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.nowFunc, b.tz)
		if err != nil {
			errCh <- err
			continue
		}

		nextCh := make(chan string, len(files))

		for _, f := range files {
			wg.Add(1)
			go compressAndRemove(f.fullPath(), &wg, errCh, nextCh)
		}

		go func() {
			for f := range nextCh {
				go func(filepath string) {
					if err := b.uploader.Upload(filepath); err != nil {
						errCh <- err
					}

					if err := os.Remove(filepath); err != nil {
						errCh <- err
					}
				}(f)
			}
		}()

		wg.Wait()
		close(nextCh)
	}
}
