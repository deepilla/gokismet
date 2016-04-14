package gokismet_test

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/deepilla/gokismet"
)

// EXAMPLE: Logging HTTP requests and responses.

// A ClientWriter is a wrapper around a gokismet Client that
// itself satisfies the Client interface. Its Do method writes
// HTTP requests and responses to the provided Writer.
type ClientWriter struct {
	client gokismet.Client
	writer io.Writer
}

// Do logs the incoming request, then calls the Do method of
// the wrapped Client and logs the response.
func (cw ClientWriter) Do(req *http.Request) (*http.Response, error) {

	cw.writeRequest(req)
	resp, err := cw.client.Do(req)
	cw.writeResponse(resp)

	return resp, err
}

func (cw ClientWriter) writeRequest(req *http.Request) {
	// Write req to cw.writer...
}

func (cw ClientWriter) writeResponse(resp *http.Response) {
	// Write resp to cw.writer...
}

func ExampleNewAPIWithClient() {

	// Wrap the default HTTP client in a ClientWriter.
	client := ClientWriter{
		http.DefaultClient,
		os.Stdout,
	}

	// Initialise an API that uses the ClientWriter.
	api := gokismet.NewAPIWithClient("YOUR_API_KEY", "http://your.website.com", client)

	comment := gokismet.Comment{
	// Comment data goes here...
	}

	// API calls now log HTTP requests/responses to stdout.
	status, err := api.CheckComment(comment.Values())

	fmt.Println(status, err)
}
