package gokismet_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/deepilla/gokismet"
)

// An Adapter is a function that takes an existing Client
// and supplements it with additional functionality.
type Adapter func(gokismet.Client) gokismet.Client

// adapt applies a series of Adapters to an existing Client.
func adapt(client gokismet.Client, adapters ...Adapter) gokismet.Client {
	for _, adapter := range adapters {
		client = adapter(client)
	}
	return client
}

// withHeader returns an Adapter that sets a custom header
// on outgoing HTTP requests before executing them.
func withHeader(key, value string) Adapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set(key, value)
			return client.Do(req)
		})
	}
}

// withRequestWriter returns an Adapter that logs outgoing
// HTTP requests to a Writer before executing them.
func withRequestWriter(writer io.Writer) Adapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {

			buf, err := httputil.DumpRequestOut(req, true)
			if err != nil {
				return nil, err
			}

			_, err = writer.Write(buf)
			if err != nil {
				return nil, err
			}

			return client.Do(req)
		})
	}
}

func ExampleClientFunc_adapter() {

	comment := gokismet.Comment{
	// Content goes here...
	}

	// Create a Client that adds custom headers and logging
	// to outgoing HTTP requests.
	client := adapt(http.DefaultClient,
		withRequestWriter(os.Stdout),
		withHeader("User-Agent", "MyApplication/1.0 | "+gokismet.UserAgent),
		withHeader("Cache-Control", "no-cache"),
	)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://your-website.com", client)

	// The Checker's HTTP requests now include the custom
	// headers and are written to stdout.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
