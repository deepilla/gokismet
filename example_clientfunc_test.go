package gokismet_test

import (
	"fmt"
	"net/http"

	"github.com/deepilla/gokismet"
)

// ClientWithUserAgent takes a user agent and returns a
// Client that applies that user agent to outgoing HTTP
// requests.
func ClientWithUserAgent(ua string) gokismet.Client {
	return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
		req.Header.Set("User-Agent", ua)
		return http.DefaultClient.Do(req)
	})
}

func ExampleClientFunc() {

	comment := gokismet.Comment{
	// Content goes here...
	}

	// Create a Client with a custom user agent.
	client := ClientWithUserAgent("YourApp/1.0 | " + gokismet.UserAgent)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://your-website.com", client)

	// The Checker's HTTP requests now include our
	// custom user agent.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
