package juggler

import (
	"github.com/pkg/errors"
	"os"
	"syscall"
)

var osChown = os.Chown

func chown(dst string, fi os.FileInfo) error {
	f, err := os.Open(dst)
	if err != nil {
		return errors.Wrapf(err, "could not open file %s to change ownership on it", dst)
	}

	_ = f.Close()

	stat := fi.Sys().(*syscall.Stat_t)

	return osChown(dst, int(stat.Uid), int(stat.Gid))
}
