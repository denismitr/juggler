// +build !linux

package juggler

import "os"

func chown(_ string, _ os.FileInfo) error {
	return nil
}
