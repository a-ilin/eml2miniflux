package eml2miniflux

import "os"

// isDirectory determines if a file represented
// by `path` is a directory or not
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
