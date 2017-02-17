package gokismet_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/deepilla/gokismet"
)

// An Adapter is a function that takes and returns a Client.
// It's used to add functionality to an existing Client.
type Adapter func(gokismet.Client) gokismet.Client

// adapt applies a series of Adapters to an existing Client.
func adapt(client gokismet.Client, adapters ...Adapter) gokismet.Client {
	for _, adapter := range adapters {
		client = adapter(client)
	}
	return client
}

// withHeader returns an Adapter that sets a header on the
// outgoing HTTP request before executing it.
func withHeader(key, value string) Adapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set(key, value)
			return client.Do(req)
		})
	}
}

// withContext returns an Adapter that sets a context value
// on the outgoing HTTP request before executing it.
func withContext(key, value string) Adapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
			ctx := context.WithValue(req.Context(), key, value)
			return client.Do(req.WithContext(ctx))
		})
	}
}

func ExampleClientFunc_adapter() {

	comment := gokismet.Comment{
	// comment data goes here
	}

	// Create a custom Client that modifies the headers
	// and context values of outgoing HTTP requests.
	client := adapt(http.DefaultClient,
		withHeader("User-Agent", "MyApplication/1.0 | "+gokismet.UserAgent),
		withHeader("Cache-Control", "no-cache"),
		withContext("SessionID", "6543210"),
	)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://example.com", client)

	// The Checker's HTTP requests now include the custom
	// headers and context values.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
