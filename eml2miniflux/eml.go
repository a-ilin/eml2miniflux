package eml2miniflux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sg3des/eml"
	"miniflux.app/model"
	"miniflux.app/storage"
)

func loadEML(filePath string) (*eml.Message, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %s", err)
	}

	message, err := eml.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("cannot parse EML: %s", err)
	}

	return &message, nil
}

func emlToEntry(store *storage.Storage, feedHelper *FeedHelper, messageFile string, user *model.User, defaultFeed *model.Feed) (*model.Entry, error) {
	// Load EML
	message, err := loadEML(messageFile)
	if err != nil {
		return nil, fmt.Errorf("cannot parse EML: %s", err)
	}

	return CreateEntryForEML(message, store, feedHelper, user, defaultFeed)
}

// Recursively traverse directories and load *.eml files
func emlWalkFunc(entries *model.Entries, entryCounter *int, store *storage.Storage, feedHelper *FeedHelper, user *model.User, defaultFeed *model.Feed) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "FS Error: %s: %s\n", path, err)
			return nil
		}

		if !info.IsDir() {
			if strings.HasSuffix(strings.ToLower(path), ".eml") {
				*entryCounter++
				if *entryCounter%1000 == 0 {
					fmt.Fprintf(os.Stdout, "Reading EML: %d\n", entryCounter)
				}

				var entry *model.Entry
				entry, err = emlToEntry(store, feedHelper, path, user, defaultFeed)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error on processing file: %s: %s\n", path, err)
				} else {
					*entries = append(*entries, entry)
				}
			}
		}

		return nil
	}
}

// Load EML from the specified messagesPath and create model Entry
// - if messagesPath is a directory: traverse recursively and load all *.eml files
// - otherwise load a single file
func GetEntriesForEML(store *storage.Storage, feedHelper *FeedHelper, messagesPath string, user *model.User, defaultFeed *model.Feed) (model.Entries, error) {
	var err error
	entries := model.Entries{}

	isDir, err := isDirectory(messagesPath)
	if err != nil {
		return entries, fmt.Errorf("cannot read path: %s %v", messagesPath, err)
	}

	entryCounter := 0

	if isDir {
		err = filepath.Walk(messagesPath, emlWalkFunc(&entries, &entryCounter, store, feedHelper, user, defaultFeed))
		fmt.Fprintf(os.Stdout, "Reading EML completed. Read EML: %d\n", entryCounter)
	} else {
		var entry *model.Entry
		entry, err = emlToEntry(store, feedHelper, messagesPath, user, defaultFeed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error on processing file: %s: %s\n", messagesPath, err)
		} else {
			entries = append(entries, entry)
		}
	}

	return entries, err
}
