//go:build !darwin && !linux

package workflow

import (
	"errors"
	"os"
)

func acquireStoreLock(string) (*os.File, error) {
	return nil, errors.New("workflow store locking is supported only on darwin and linux")
}

func releaseStoreLock(*os.File) error {
	return nil
}

func openRegularNoFollow(string, int, os.FileMode) (*os.File, error) {
	return nil, errors.New("workflow files are supported only on darwin and linux")
}
