package eml2miniflux

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rylans/getlang"
	"github.com/sg3des/eml"
	"miniflux.app/crypto"
	"miniflux.app/model"
	"miniflux.app/reader/rewrite"
	"miniflux.app/reader/sanitizer"
	"miniflux.app/storage"
)

var (
	bodyRx                    = regexp.MustCompile(`(?s)<body(?:[^>]*)?>(.*)<\/body>`)
	feedEntryContentRx        = regexp.MustCompile(`(?s)div\s+class="feedEntryContent">\s*(.*)<\/div>\s*<div\s+class="feedEntryLinks">`)
	feedEntryAlternateLinksRx = regexp.MustCompile(`(?s)<ul\s+class="feedEntryAlternateLinks">\s*<li>\s*<a\s+href="([^"]+)"`)
)

func CreateEntryForEML(message *eml.Message, store *storage.Storage, feedHelper *FeedHelper, user *model.User, defaultFeed *model.Feed) (*model.Entry, error) {
	entry := model.Entry{
		Status:     model.EntryStatusUnread,
		Hash:       entryHash(message),
		Title:      message.Subject,
		URL:        entryUrl(message),
		Date:       message.Date,
		CreatedAt:  message.ReceivedDate,
		ChangedAt:  message.ReceivedDate,
		Enclosures: make(model.EnclosureList, 0),
		Tags:       message.Keywords,
	}

	if !message.ReceivedDate.IsZero() {
		entry.CreatedAt = message.ReceivedDate
		entry.ChangedAt = message.ReceivedDate
	} else {
		entry.CreatedAt = message.Date
		entry.ChangedAt = message.Date
	}

	if message.Sender != nil {
		entry.Author = message.Sender.String()
	}

	if len(entry.Title) == 0 {
		entry.Title = sanitizer.TruncateHTML(entry.Content, 100)
	}

	if len(entry.Title) == 0 {
		entry.Title = entry.URL
	}

	if len(message.Html) > 0 {
		entry.Content = extractBody(message.Html)
	} else {
		entry.Content = message.Text
	}

	// Assign User & Feed
	err := assignUserFeed(&entry, store, user, feedHelper, defaultFeed)
	if err != nil {
		if _, ok := err.(*FeedIgnoreError); ok {
			// silently ignore
			return nil, err
		}
		return nil, fmt.Errorf("unable to assign feed to entry: %v", err)
	}

	// Rewrite and sanitize content
	rewriteEntry(&entry, user)

	return &entry, nil
}

func assignUserFeed(entry *model.Entry, store *storage.Storage, user *model.User, feedHelper *FeedHelper, defaultFeed *model.Feed) error {
	var err error

	feed := defaultFeed
	if defaultFeed == nil {
		feed, err = feedHelper.FeedForEntryUrl(entry.URL)
		if err != nil {
			return err
		}
	}

	entry.UserID = user.ID
	entry.FeedID = feed.ID
	entry.Feed = feed

	return nil
}

func rewriteEntry(entry *model.Entry, user *model.User) {
	entry.Content = rewrite.Rewriter(entry.URL, entry.Content, entry.Feed.RewriteRules)
	entry.Content = strings.TrimSpace(sanitizer.Sanitize(entry.URL, entry.Content))
	entry.ReadingTime = calculateReadingTime(entry.Content, user)
}

func extractBody(content string) string {
	for _, reg := range []*regexp.Regexp{feedEntryContentRx, bodyRx} {
		match := reg.FindStringSubmatch(content)
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
	}

	return content
}

func entryUrl(message *eml.Message) string {
	if len(message.ContentBase) > 0 {
		// Thunderbird normally sets ContentBase
		return message.ContentBase
	}

	// Use `feedEntryAlternateLinks` token
	match := feedEntryAlternateLinksRx.FindStringSubmatch(message.Html)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

func entryHash(message *eml.Message) string {
	// remove suffix added by Thunderbird
	msgId := strings.TrimSuffix(message.MessageId, "@localhost.localdomain")
	if len(msgId) > 0 {
		return crypto.Hash(msgId)
	}

	if len(message.ContentBase) > 0 {
		return crypto.Hash(message.ContentBase)
	}

	return ""
}

// Copy-pasted from `processor.go` due to not being exported
func calculateReadingTime(content string, user *model.User) int {
	sanitizedContent := sanitizer.StripTags(content)
	languageInfo := getlang.FromString(sanitizedContent)

	var timeToReadInt int
	if languageInfo.LanguageCode() == "ko" || languageInfo.LanguageCode() == "zh" || languageInfo.LanguageCode() == "jp" {
		timeToReadInt = int(math.Ceil(float64(utf8.RuneCountInString(sanitizedContent)) / float64(user.CJKReadingSpeed)))
	} else {
		nbOfWords := len(strings.Fields(sanitizedContent))
		timeToReadInt = int(math.Ceil(float64(nbOfWords) / float64(user.DefaultReadingSpeed)))
	}

	return timeToReadInt
}
