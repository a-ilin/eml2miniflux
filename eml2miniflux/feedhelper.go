package eml2miniflux

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"miniflux.app/model"
	"miniflux.app/storage"
)

type FeedHelper struct {
	feedsLookup map[string]*model.Feed
	feedsUrl    map[string]*model.Feed
	feedsId     map[int64]*model.Feed
}

func CreateFeedHelper(store *storage.Storage, user *model.User) (*FeedHelper, error) {
	var helper FeedHelper

	err := helper.loadAllFeeds(store, user)
	if err != nil {
		return nil, err
	}

	return &helper, nil
}

func (h *FeedHelper) loadAllFeeds(store *storage.Storage, user *model.User) error {
	allFeeds, err := store.Feeds(user.ID)
	if err != nil {
		return fmt.Errorf(`cannot load feeds from DB: %s`, err)
	}

	h.feedsUrl = make(map[string]*model.Feed)
	for _, feed := range allFeeds {
		h.feedsUrl[feed.FeedURL] = feed
	}

	h.feedsId = make(map[int64]*model.Feed)
	for _, feed := range allFeeds {
		h.feedsId[feed.ID] = feed
	}

	return err
}

func (h *FeedHelper) LoadMap(fileName string) error {
	h.feedsLookup = make(map[string]*model.Feed)

	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("cannot open feed helper file: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		err = h.processConfigLine(line)
		if err != nil {
			return fmt.Errorf(`wrong feed helper line #%d: %s: %s`, lineNum, err, line)
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("cannot read feed helper file: %s", err)
	}

	return nil
}

func (h *FeedHelper) processConfigLine(line string) error {
	line = strings.TrimSpace(line)

	if len(line) == 0 {
		return nil
	}

	if strings.HasPrefix(line, `#`) {
		// comment
		return nil
	}

	parts := strings.Split(string(line), `=>`)
	if len(parts) != 2 {
		return fmt.Errorf(`separator => is missing`)
	}

	entryUrl := strings.TrimSpace(parts[0])
	if len(entryUrl) == 0 {
		return fmt.Errorf(`entry URL is missing`)
	}

	feedUrl := strings.TrimSpace(parts[1])
	if len(feedUrl) == 0 {
		return fmt.Errorf(`feed URL is missing`)
	}

	if feed, ok := h.feedsUrl[feedUrl]; ok {
		h.feedsLookup[entryUrl] = feed
	} else {
		return fmt.Errorf(`cannot find feed with URL: %s`, feedUrl)
	}

	return nil
}

func (h *FeedHelper) FeedForEntryUrl(entryUrl string) *model.Feed {
	for e, f := range h.feedsLookup {
		if strings.Contains(entryUrl, e) {
			return f
		}
	}

	return nil
}

func (h *FeedHelper) FeedByID(feedId int64) *model.Feed {
	feed := h.feedsId[feedId]
	return feed
}

func (h *FeedHelper) FeedByURL(feedUrl string) *model.Feed {
	feed := h.feedsUrl[feedUrl]
	return feed
}
