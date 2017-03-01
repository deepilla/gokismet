// This code example uses the Context package, introduced
// in Go 1.7. To prevent older versions of Go choking on
// the Context code, this file is excluded from pre-1.7
// builds.

// +build go1.7

package gokismet_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/deepilla/gokismet"
)

type contextKey string

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

// withHeader returns an Adapter that adds a custom header
// to outgoing HTTP requests.
func withHeader(key, value string) Adapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set(key, value)
			return client.Do(req)
		})
	}
}

// withContext returns an Adapter that adds a custom context
// value to outgoing HTTP requests.
func withContext(key, value string) Adapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
			ctx := context.WithValue(req.Context(), contextKey(key), value)
			return client.Do(req.WithContext(ctx))
		})
	}
}

func ExampleClientFunc_adapter() {

	comment := gokismet.Comment{
	// Content goes here...
	}

	// Create a Client that adds custom headers and context
	// values to outgoing HTTP requests.
	client := adapt(http.DefaultClient,
		withHeader("User-Agent", "YourApp/1.0 | "+gokismet.UserAgent),
		withHeader("Cache-Control", "no-cache"),
		withContext("SessionID", "1234567890"),
	)

	// Create a Checker that uses the Client.
	ch := gokismet.NewCheckerClient("YOUR-API-KEY", "http://your-website.com", client)

	// The Checker's HTTP requests now include the custom
	// headers and context values.
	status, err := ch.Check(comment.Values())

	fmt.Println(status, err)
}
