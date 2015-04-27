package gokismet_test

import (
	"fmt"
	"net/url"

	"github.com/deepilla/gokismet"
)

func ExampleAPI() {

	// Example: Using API to check a comment for spam

	// Create a new API
	api := &gokismet.API{}

	// Verify your API key with Akismet
	err := api.VerifyKey("YOUR_API_KEY", "http://yourwebsite.com")
	if err != nil {
		// Handle the error
		fmt.Println(err)
		return
	}

	// Set up your query parameters
	params := url.Values{
		"blog":                 {"http://yourwebsite.com"},
		"user_ip":              {"127.0.0.1"},
		"user_agent":           {"Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6"},
		"permalink":            {"http://www.yourwebsite.com/2015/05/05/its-cinco-de-mayo/"},
		"comment_type":         {"comment"},
		"comment_author":       {"A. Commenter"},
		"comment_author_email": {"acommenter@aol.com"},
		"comment_author_url":   {"http://www.lovecincodemayo.com"},
		"comment_content":      {"I love Cinco de Mayo!"},
	}

	// Call CheckComment
	status, err := api.CheckComment(&params)
	if err != nil {
		// Handle the error
		fmt.Println(err)
		return
	}

	// Do something based on the returned status
	switch status {
	case gokismet.StatusNotSpam:
		fmt.Println("Akismet thinks this is a legit comment")
	case gokismet.StatusProbableSpam:
		fmt.Println("Akismet thinks this is spam")
	case gokismet.StatusDefiniteSpam:
		fmt.Println("Akismet thinks this is the worst kind of spam")
	}
}

func ExampleComment() {

	// Example: Using Comment to check a comment for spam

	// Create a new Comment
	comment, err := gokismet.NewComment("YOUR_API_KEY", "http://www.yourwebsite.com")
	if err != nil {
		// Handle the error
		fmt.Println(err)
		return
	}

	// Set your comment data
	comment.SetUserIP("127.0.0.1")
	comment.SetUserAgent("Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6")
	comment.SetPage("http://www.yourwebsite.com/2015/05/05/its-cinco-de-mayo/")
	comment.SetAuthor("A. Commenter")
	comment.SetEmail("acommenter@aol.com")
	comment.SetURL("http://www.lovecincodemayo.com")
	comment.SetContent("I love Cinco de Mayo!")

	// Call Check
	status, err := comment.Check()
	if err != nil {
		// Handle the error
		fmt.Println(err)
		return
	}

	// Do something based on the returned status
	switch status {
	case gokismet.StatusNotSpam:
		fmt.Println("Akismet thinks this is a legit comment")
	case gokismet.StatusProbableSpam:
		fmt.Println("Akismet thinks this is spam")
	case gokismet.StatusDefiniteSpam:
		fmt.Println("Akismet thinks this is the worst kind of spam")
	}
}
