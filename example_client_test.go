package gokismet_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/deepilla/gokismet"
)

// A RequestWriterClient is a Client that logs outgoing
// requests to a Writer.
type RequestWriterClient struct {
	client gokismet.Client
	writer io.Writer
}

// Do logs and executes outgoing requests.
//
// Note: For simple cases like this, where a Client's
// custom behaviour is contained in a single function,
// consider using a ClientFunc instead of a type.
func (rw *RequestWriterClient) Do(req *http.Request) (*http.Response, error) {

	buf, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return nil, err
	}

	_, err = rw.writer.Write(buf)
	if err != nil {
		return nil, err
	}

	return rw.client.Do(req)
}

func ExampleClient() {

	comment := gokismet.Comment{
	// Content goes here...
	}

	// Create a RequestWriterClient that uses the default
	// HTTP client and writes to stdout.
	client := &RequestWriterClient{
		http.DefaultClient,
		os.Stdout,
	}

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://your-website.com", client)

	// The Checker's HTTP requests are now written to stdout.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
