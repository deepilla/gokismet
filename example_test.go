package gokismet_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/deepilla/gokismet"
)

func ExampleAPI_CheckComment() {
	// Define the comment data.
	comment := gokismet.Comment{
		UserIP:        "127.0.0.1",
		UserAgent:     "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		Page:          "http://your.website.com/2016/05/05/its-cinco-de-mayo/",
		PageTimestamp: time.Date(2016, time.May, 5, 10, 30, 0, 0, time.UTC),
		Author:        "A. Commenter",
		AuthorEmail:   "acommenter@aol.com",
		Content:       "I love Cinco de Mayo!",
		// etc...
	}

	// Create an API instance.
	api := gokismet.NewAPI("YOUR_API_KEY", "http://your.website.com")

	// Call CheckComment.
	status, err := api.CheckComment(comment.Values())

	fmt.Println(status, err)
}

func ExampleWrapClient() {
	// EXAMPLE: Overwriting gokismet's default user agent.

	// Define some custom request headers.
	headers := map[string]string{
		"User-Agent": "MyApplication/1.0 | " + gokismet.UA,
	}

	// Wrap the default HTTP client with our headers.
	client := gokismet.WrapClient(http.DefaultClient, headers)

	// Initialise an API that uses our client.
	api := gokismet.NewAPIWithClient("YOUR_API_KEY", "http://your.website.com", client)

	comment := gokismet.Comment{
	// Comment data goes here...
	}

	// API calls now have User Agent "MyApplication/1.0 | Gokismet/2.0".
	status, err := api.CheckComment(comment.Values())

	fmt.Println(status, err)
}
