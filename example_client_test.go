package gokismet_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/deepilla/gokismet"
)

// A RequestWriterClient is a Client that wraps an existing
// Client and writes outgoing requests to a Writer.
type RequestWriterClient struct {
	client gokismet.Client
	writer io.Writer
}

// NewRequestWriterClient creates a RequestWriterClient.
func NewRequestWriterClient(client gokismet.Client, writer io.Writer) *RequestWriterClient {

	if client == nil {
		client = http.DefaultClient
	}

	if writer == nil {
		writer = os.Stdout
	}

	return &RequestWriterClient{
		client,
		writer,
	}
}

// Do writes the outgoing request to the specified Writer
// before executing it.
func (wc *RequestWriterClient) Do(req *http.Request) (*http.Response, error) {

	buf, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return nil, err
	}

	_, err = wc.writer.Write(buf)
	if err != nil {
		return nil, err
	}

	return wc.client.Do(req)
}

func ExampleClient() {

	comment := gokismet.Comment{
	// comment data goes here
	}

	// Create a RequestWriterClient that uses the default
	// HTTP client and writes to stdout.
	client := NewRequestWriterClient(nil, nil)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://example.com", client)

	// The Checker's HTTP requests are now written to stdout.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
