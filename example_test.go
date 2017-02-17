package gokismet_test

import (
	"fmt"
	"time"

	"github.com/deepilla/gokismet"
)

func ExampleChecker_Check() {
	// Comment data defined as key-value pairs. See the Akismet
	// docs for a list of valid keys. Or better still, use the
	// Comment type instead.
	values := map[string]string{
		"user_ip":                   "127.0.0.1",
		"user_agent":                "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		"permalink":                 "http://example.com/posts/feliz-cinco-de-mayo/",
		"comment_post_modified_gmt": "2016-05-05T10:30:00Z",
		"comment_author":            "A. Commenter",
		"comment_author_email":      "acommenter@aol.com",
		"comment_content":           "I love Cinco de Mayo!",
		// etc...
	}

	ch := gokismet.NewChecker("YOUR-API-KEY", "http://example.com")

	status, err := ch.Check(values)

	switch status {
	case gokismet.StatusHam:
		fmt.Println("Comment is legit")
	case gokismet.StatusProbableSpam, gokismet.StatusDefiniteSpam:
		fmt.Println("Comment is spam")
	case gokismet.StatusUnknown:
		fmt.Println("Something went wrong:", err)
	}
}

func ExampleChecker_Check_comment() {
	// Comment data defined with the Comment type. This is less
	// error-prone than using a map.
	comment := gokismet.Comment{
		UserIP:        "127.0.0.1",
		UserAgent:     "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		Page:          "http://example.com/posts/feliz-cinco-de-mayo/",
		PageTimestamp: time.Date(2016, time.May, 5, 10, 30, 0, 0, time.UTC),
		Author:        "A. Commenter",
		AuthorEmail:   "acommenter@aol.com",
		Content:       "I love Cinco de Mayo!",
		// etc...
	}

	ch := gokismet.NewChecker("YOUR-API-KEY", "http://example.com")

	status, err := ch.Check(comment.Values())

	switch status {
	case gokismet.StatusHam:
		fmt.Println("Comment is legit")
	case gokismet.StatusProbableSpam, gokismet.StatusDefiniteSpam:
		fmt.Println("Comment is spam")
	case gokismet.StatusUnknown:
		fmt.Println("Something went wrong:", err)
	}
}
