package gokismet_test

import (
	"fmt"
	"time"

	"github.com/deepilla/gokismet"
)

func ExampleChecker_Check() {
	// This example defines content as a map of key-value pairs
	// (see the Akismet docs for a list of valid keys). Gokismet
	// also provides a Comment type that generates this map for you.
	values := map[string]string{
		"user_ip":                   "127.0.0.1",
		"user_agent":                "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		"permalink":                 "http://your-website.com/posts/feliz-cinco-de-mayo/",
		"comment_post_modified_gmt": "2016-05-05T10:30:00Z",
		"comment_author":            "A. Commenter",
		"comment_author_email":      "acommenter@aol.com",
		"comment_content":           "I love Cinco de Mayo!",
		// etc...
	}

	ch := gokismet.NewChecker("YOUR-API-KEY", "http://your-website.com")

	status, err := ch.Check(values)

	switch status {
	case gokismet.StatusHam:
		fmt.Println("This is legit content")
	case gokismet.StatusProbableSpam, gokismet.StatusDefiniteSpam:
		fmt.Println("This is spam")
	case gokismet.StatusUnknown:
		fmt.Println("Something went wrong:", err)
	}
}

func ExampleChecker_Check_comment() {
	// This example uses the Comment type to define content.
	// You can also use a map of key-value pairs.
	comment := gokismet.Comment{
		UserIP:        "127.0.0.1",
		UserAgent:     "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		Page:          "http://your-website.com/posts/feliz-cinco-de-mayo/",
		PageTimestamp: time.Date(2016, time.May, 5, 10, 30, 0, 0, time.UTC),
		Author:        "A. Commenter",
		AuthorEmail:   "acommenter@aol.com",
		Content:       "I love Cinco de Mayo!",
		// etc...
	}

	ch := gokismet.NewChecker("YOUR-API-KEY", "http://your-website.com")

	status, err := ch.Check(comment.Values())

	switch status {
	case gokismet.StatusHam:
		fmt.Println("This is legit content")
	case gokismet.StatusProbableSpam, gokismet.StatusDefiniteSpam:
		fmt.Println("This is spam")
	case gokismet.StatusUnknown:
		fmt.Println("Something went wrong:", err)
	}
}
