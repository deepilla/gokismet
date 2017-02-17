package gokismet_test

import (
	"fmt"
	"net/http"

	"github.com/deepilla/gokismet"
)

// do is a standalone function that modifies the header of the
// outgoing HTTP request before executing it. Note that the
// function signature matches ClientFunc.
func do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "MyApplication/1.0 | "+gokismet.UserAgent)
	return http.DefaultClient.Do(req)
}

func ExampleClientFunc() {

	comment := gokismet.Comment{
	// comment data goes here
	}

	// Convert the do function into a gokismet Client by casting
	// it to a ClientFunc. We can do this with any function that
	// has this function signature.
	client := gokismet.ClientFunc(do)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://example.com", client)

	// The Checker's HTTP requests are now executed via the do
	// function. As such they will include the modified header.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
