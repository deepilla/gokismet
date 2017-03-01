package gokismet_test

import (
	"fmt"
	"net/http"

	"github.com/deepilla/gokismet"
)

// A UserAgentClient is a Client that applies a custom
// user agent to outgoing HTTP requests.
type UserAgentClient string

// Do sets the User-Agent header on the outgoing request
// before executing it with the default HTTP client.
//
// Note: For simple cases like this, where a custom
// Client's behaviour is contained in a single function,
// consider using a ClientFunc instead of a type.
func (ua UserAgentClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", string(ua))
	return http.DefaultClient.Do(req)
}

func ExampleClient() {

	comment := gokismet.Comment{
	// Content goes here...
	}

	// Create a UserAgentClient from a user agent string.
	client := UserAgentClient("YourApp/1.0 | " + gokismet.UserAgent)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://your-website.com", client)

	// The Checker's HTTP requests now include our custom
	// user agent.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
