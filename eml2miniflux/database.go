package eml2miniflux

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/lib/pq"
	"miniflux.app/model"
	"miniflux.app/storage"
)

type DatabaseProcessor struct {
	Db        *sql.DB
	Store     *storage.Storage
	BatchSize int
	Retries   int
}

type databaseProcessorFunc func(entries model.Entries) error

func (p *DatabaseProcessor) databaseProcessorRun(proc databaseProcessorFunc, allEntries model.Entries) error {
	var err error
	var batch model.Entries
	var entryCounter int

	for len(allEntries) > 0 {
		if p.BatchSize >= len(allEntries) {
			batch = allEntries
			allEntries = make(model.Entries, 0)
		} else {
			batch = allEntries[0:p.BatchSize]
			allEntries = allEntries[p.BatchSize:]
		}

		for retry := 0; retry < p.Retries; retry++ {
			err = proc(batch)
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
		fmt.Fprintf(os.Stdout, "Processed entries (DB): %d\n", entryCounter)
	}

	return nil
}

func (p *DatabaseProcessor) UpdateStorageEntries(allEntries model.Entries, overwrite bool) error {
	proc := func(batch model.Entries) error {
		if len(batch) == 0 {
			return nil
		}

		userID := batch[0].UserID
		feedID := batch[0].FeedID

		return p.Store.RefreshFeedEntries(userID, feedID, batch, overwrite)
	}

	return p.databaseProcessorRun(proc, allEntries)
}

func (p *DatabaseProcessor) RemoveStorageEntries(allEntries model.Entries) error {
	proc := func(batch model.Entries) error {
		if len(batch) == 0 {
			return nil
		}

		userID := batch[0].UserID
		entryHashes := make([]string, len(batch))

		for i, entry := range batch {
			entryHashes[i] = entry.Hash
		}

		return p.deleteEntriesByHash(userID, entryHashes)
	}

	return p.databaseProcessorRun(proc, allEntries)
}

func (p *DatabaseProcessor) deleteEntriesByHash(userID int64, entryHashes []string) error {
	query := `
		DELETE FROM
			entries
		WHERE
			user_id=$1
		AND
			id IN (SELECT id FROM entries WHERE user_id=$2 AND (hash=ANY($3)))
	`
	if _, err := p.Db.Exec(query, userID, userID, pq.Array(entryHashes)); err != nil {
		return fmt.Errorf(`unable to remove entries: %v`, err)
	}

	return nil
}
