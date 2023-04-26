package eml2miniflux

import (
	"fmt"
	"os"
	"time"

	"miniflux.app/model"
	"miniflux.app/storage"
)

func UpdateStorageEntries(entries model.Entries, store *storage.Storage, batchSize int, retries int, overwrite bool) error {
	if len(entries) == 0 {
		return nil
	}

	userID := entries[0].UserID
	feedID := entries[0].FeedID

	var err error
	var batch model.Entries
	entryCounter := 0
	for len(entries) > 0 {
		if batchSize >= len(entries) {
			batch = entries
			entries = make(model.Entries, 0)
		} else {
			batch = entries[0:batchSize]
			entries = entries[batchSize:]
		}

		for retry := 0 ; retry < retries; retry++ {
			err = store.RefreshFeedEntries(userID, feedID, batch, overwrite)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Database transaction failed: %s. Retrying...\n", err)
				time.Sleep(10 * time.Second)
			} else {
				break
			}
		}
		if err != nil {
			return err
		}

		entryCounter += len(batch)
		fmt.Fprintf(os.Stdout, "Commited to DB: %d\n", entryCounter)
	}

	return nil
}
