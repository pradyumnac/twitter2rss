package twitter2rss

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/feeds"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

var (
	// Index html for empty requests
	Index string
	// Global mutex
	mu sync.Mutex
)

// Twitter2RSS return RSS from twitter timeline
func Twitter2RSS(screenName string, count int, excludeReplies bool) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	feed := &feeds.Feed{
		Title:       "Twitter feed @" + screenName,
		Link:        &feeds.Link{Href: "https://twitter.com/" + screenName},
		Description: "Twitter feed @" + screenName + " through Twitter to RSS proxy by Nomadic",
	}

	for tweet := range twitterscraper.WithReplies(!excludeReplies).GetTweets(context.Background(), screenName, count) {
		if tweet.Error != nil {
			return "", tweet.Error
		}

		if excludeReplies && tweet.IsRetweet {
			continue
		}

		if tweet.TimeParsed.After(feed.Created) {
			feed.Created = tweet.TimeParsed
		}

		var title string

		titleSplit := strings.FieldsFunc(tweet.Text, func(r rune) bool {
			return r == '\n' || r == '!' || r == '?' || r == ':' || r == '<' || r == '.' || r == ','
		})
		if len(titleSplit) > 0 {
			if strings.HasPrefix(titleSplit[0], "a href") || strings.HasPrefix(titleSplit[0], "http") {
				title = "link"
			} else {
				title = titleSplit[0]
			}
		}
		title = strings.TrimSuffix(title, "https")
		title = strings.TrimSpace(title)

		feed.Add(&feeds.Item{
			Author:      &feeds.Author{Name: screenName},
			Created:     tweet.TimeParsed,
			Description: tweet.HTML,
			Id:          tweet.PermanentURL,
			Link:        &feeds.Link{Href: tweet.PermanentURL},
			Title:       title,
		})
	}

	if len(feed.Items) == 0 {
		return "", fmt.Errorf("tweets not found")
	}

	return feed.ToRss()
}

// HTTPHandler function
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = strings.Trim(html.EscapeString(r.URL.Path), "/")
	}
	if name != "" {
		pageCount, _ := strconv.Atoi(r.URL.Query().Get("pages"))
		statusCount, _ := strconv.Atoi(r.URL.Query().Get("count"))
		if pageCount > 0 && statusCount == 0 {
			statusCount = pageCount * 10
		}
		if statusCount == 0 {
			statusCount = 10
		} else if statusCount > 100 {
			statusCount = 100
		}

		excludeReplies := r.URL.Query().Get("exclude_replies") == "on"

		log.Printf("Process timeline @%s (count: %d, exclude_replies: %v)", name, statusCount, excludeReplies)
		rss, err := Twitter2RSS(name, statusCount, excludeReplies)
		if err != nil {
			log.Printf("Error timeline @%s: %s\n", name, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rss))
	} else {
		w.Write([]byte(Index))
	}
}
