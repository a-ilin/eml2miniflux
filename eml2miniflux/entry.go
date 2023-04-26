package eml2miniflux

import (
	"math"
	"regexp"
	"strings"
	"time"
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
		Title:      message.Subject,
		URL:        entryUrl(message),
		Date:       message.Date,
		CreatedAt:  message.ReceivedDate,
		ChangedAt:  message.ReceivedDate,
		Enclosures: make(model.EnclosureList, 0),
		Tags:       message.Keywords,
	}

	entry.Hash = entryHash(message, entry.URL)

	if !message.ReceivedDate.IsZero() {
		entry.CreatedAt = message.ReceivedDate
		entry.ChangedAt = message.ReceivedDate
	} else {
		entry.CreatedAt = time.Now()
		entry.ChangedAt = time.Now()
	}

	// Some messages do not contain publication date, which is then being set to current time
	if entry.Date.After(entry.CreatedAt) {
		entry.Date = entry.CreatedAt
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
	feed, err := assignUserFeed(&entry, store, user, feedHelper, defaultFeed)
	if err != nil {
		return nil, err
	}

	// Rewrite and sanitize content
	rewriteEntry(&entry, user, feed)

	return &entry, nil
}

func assignUserFeed(entry *model.Entry, store *storage.Storage, user *model.User, feedHelper *FeedHelper, defaultFeed *model.Feed) (*model.Feed, error) {
	var err error

	feed := defaultFeed
	if defaultFeed == nil {
		feed, err = feedHelper.FeedForEntryUrl(entry.URL)
		if err != nil {
			return nil, err
		}
	}

	entry.UserID = user.ID
	entry.FeedID = feed.ID

	return feed, nil
}

func rewriteEntry(entry *model.Entry, user *model.User, feed *model.Feed) {
	entry.Content = rewrite.Rewriter(entry.URL, entry.Content, feed.RewriteRules)
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

func entryHash(message *eml.Message, entryUrl string) string {
	// remove suffix added by Thunderbird
	msgId := strings.TrimSuffix(message.MessageId, "@localhost.localdomain")
	if len(msgId) > 0 {
		return crypto.Hash(msgId)
	}

	if len(entryUrl) > 0 {
		return crypto.Hash(entryUrl)
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
