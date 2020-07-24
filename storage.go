package juggler

import (
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
		return newCloudCompression(j.directory, j.prefix, j.uploader, j.format, j.timezone)
	}

	if j.compression {
		return newLocalCompression(j.directory, j.prefix, j.format, j.timezone)
	}

	return newLimitedStorage(j.maxBackups, j.directory, j.prefix, j.format, j.timezone)
}

type base struct {
	dir    string
	prefix string
	format *regexp.Regexp
	tz     *time.Location
}

type cloudCompression struct {
	base
	uploader uploader
}

func newCloudCompression(dir, prefix string, uploader uploader, format *regexp.Regexp, tz *time.Location) *cloudCompression {
	return &cloudCompression{
		base: base{
			dir:    dir,
			prefix: prefix,
			format: format,
			tz:     tz,
		},
		uploader: uploader,
	}
}

type localCompression struct {
	base
}

func newLocalCompression(dir, prefix string, format *regexp.Regexp, tz *time.Location) *localCompression {
	return &localCompression{
		base: base{
			dir:    dir,
			prefix: prefix,
			format: format,
			tz:     tz,
		},
	}
}

func (b *localCompression) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.tz)
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

func newLimitedStorage(maxBackups int, dir, prefix string, format *regexp.Regexp, tz *time.Location) *limitedStorage {
	return &limitedStorage{
		base: base{
			dir:    dir,
			prefix: prefix,
			format: format,
			tz:     tz,
		},
		maxBackups: maxBackups,
	}
}

func (b *limitedStorage) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.tz)
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
					errCh <- err
				}

				wg.Done()
			}(files[i])
		}

		wg.Wait()
	}
}

func (b *cloudCompression) start(runCh <-chan struct{}, errCh chan<- error) {
	for range runCh {
		var wg sync.WaitGroup

		files, err := scanBackups(b.dir, b.prefix, b.format, b.tz)
		if err != nil {
			errCh <- err
			continue
		}

		nextCh := make(chan string, len(files))

		for _, f := range files {
			wg.Add(1)
			go compressAndRemove(f.fullPath(), &wg, errCh, nil)
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
