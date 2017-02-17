package gokismet_test

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/deepilla/gokismet"
)

// To run tests against the live Akismet server, specify an
// API key and website on the command line.
// Note: We're relying on go test's flag parsing to set
// these values. Seems to work...
var flags = struct {
	APIKey *string
	Site   *string
}{
	flag.String("akismet.key", "", "your Akismet API Key"),
	flag.String("akismet.site", "", "the website associated with your Akismet API Key"),
}

const (
	TestAPIKey = "123456789abc"
	TestSite   = "http://example.com"
)

var UTCMinus5 = time.FixedZone("UTCMinus5", -5*60*60)

var (
	fnCheck      = (*gokismet.Checker).Check
	fnReportHam  = (*gokismet.Checker).ReportHam
	fnReportSpam = (*gokismet.Checker).ReportSpam
)

type (
	ErrorFunc       func(*gokismet.Checker, map[string]string) error
	StatusErrorFunc func(*gokismet.Checker, map[string]string) (gokismet.SpamStatus, error)
)

func toStatusErrorFunc(fn ErrorFunc) StatusErrorFunc {
	return func(checker *gokismet.Checker, values map[string]string) (gokismet.SpamStatus, error) {
		return gokismet.StatusUnknown, fn(checker, values)
	}
}

func toErrorFunc(fn StatusErrorFunc) ErrorFunc {
	return func(checker *gokismet.Checker, values map[string]string) error {
		_, err := fn(checker, values)
		return err
	}
}

// A RequestInfo contains the pertinent parts of an HTTP Request
// (i.e. any values that gokismet sets when making calls to Akismet).
type RequestInfo struct {
	Method      string
	URL         string
	HeaderItems map[string]string
	Body        string
}

// NewRequestInfo creates a RequestInfo from an HTTP Request.
// Note: This function consumes the Request body. Watch what
// you do with the Request afterwards.
func NewRequestInfo(req *http.Request) (*RequestInfo, error) {

	info := &RequestInfo{
		Method: req.Method,
		URL:    req.URL.String(),
	}

	for k, v := range req.Header {
		if info.HeaderItems == nil {
			info.HeaderItems = make(map[string]string)
		}
		info.HeaderItems[k] = strings.Join(v, "|")
	}

	if req.Body != nil {
		defer req.Body.Close()
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		info.Body = string(body)
	}

	return info, nil
}

// A ResponseInfo contains the pertinent parts of an HTTP Response
// (i.e. any values in the Akismet response that gokismet uses).
type ResponseInfo struct {
	StatusCode  int
	HeaderItems map[string]string
	Body        string
}

// NewResponse creates a barebones HTTP Response from a ResponseInfo.
func NewResponse(info *ResponseInfo) *http.Response {

	resp := &http.Response{
		StatusCode: info.StatusCode,
		Status:     fmt.Sprintf("%d %s", info.StatusCode, http.StatusText(info.StatusCode)),
		Body:       ioutil.NopCloser(strings.NewReader(info.Body)),
	}

	for k, v := range info.HeaderItems {
		if resp.Header == nil {
			resp.Header = make(http.Header)
		}
		resp.Header.Set(k, v)
	}

	return resp
}

// A RequestStore is a mock Client that captures the details
// of incoming requests.
type RequestStore struct {
	Requests []*RequestInfo
}

// Do stores the details of the incoming request and returns
// a nil Response.
func (rs *RequestStore) Do(req *http.Request) (*http.Response, error) {

	info, err := NewRequestInfo(req)
	if err != nil {
		return nil, err
	}

	rs.Requests = append(rs.Requests, info)

	return nil, errors.New("RequestStore always returns a Nil Response")
}

// A Responder is a mock Client that responds to incoming requests
// according to user-defined responses.
type Responder struct {
	Responses map[string]*ResponseInfo
}

// Do returns the user-defined response for the incoming request.
// If no response exists, Do returns an error.
func (r *Responder) Do(req *http.Request) (*http.Response, error) {

	// Akismet URL format is https://rest.akismet.com/1.1/verify-key.
	// The last part of the URL, as returned by path.Base, is the
	// name of the API call.
	call := path.Base(req.URL.Path)

	info := r.Responses[call]
	if info == nil {
		return nil, fmt.Errorf("No response for %q", call)
	}

	return NewResponse(info), nil
}

// AddResponses adds another Responder's user-defined responses
// to this Responder.
func (r *Responder) AddResponses(in *Responder) {
	for k, v := range in.Responses {
		if r.Responses == nil {
			r.Responses = make(map[string]*ResponseInfo)
		}
		r.Responses[k] = v
	}
}

// verifyingResponder is a Responder that verifies API keys.
var verifyingResponder = &Responder{
	map[string]*ResponseInfo{
		"verify-key": {
			Body:       "valid",
			StatusCode: http.StatusOK,
		},
	},
}

// A clientAdapter is a function that takes a Client and
// returns a Client, usually the original Client supplemented
// with additional functionality.
type clientAdapter func(gokismet.Client) gokismet.Client

// adaptClient applies a series of clientAdapters to a Client.
func adaptClient(client gokismet.Client, adapters ...clientAdapter) gokismet.Client {
	for _, adapter := range adapters {
		client = adapter(client)
	}
	return client
}

// withResponder returns a clientAdapter that calls the Do
// method of the original Client and then forwards the request
// to a Responder to generate the Response.
func withResponder(responder *Responder) clientAdapter {
	return func(client gokismet.Client) gokismet.Client {
		return gokismet.ClientFunc(func(req *http.Request) (*http.Response, error) {
			client.Do(req)
			return responder.Do(req)
		})
	}
}

// A FieldSetter uses reflection to set the fields of a struct.
type FieldSetter struct {
	val reflect.Value
}

// NewFieldSetter takes a pointer to a struct and returns
// a FieldSetter that can set the fields of that struct.
func NewFieldSetter(val interface{}) *FieldSetter {
	return &FieldSetter{reflect.Indirect(reflect.ValueOf(val))}
}

// Set sets the field with the given name to the given value.
func (c *FieldSetter) Set(name string, value interface{}) error {

	// Panics if val is not a struct. Returns a nil Value
	// if the field does not exist.
	v := c.val.FieldByName(name)

	switch {
	case !v.IsValid():
		return fmt.Errorf("Field %s not found", name)
	case !v.CanSet():
		return fmt.Errorf("Field %s is not settable", name)
	default:
		v.Set(reflect.ValueOf(value))
		return nil
	}
}

// NumFields returns the number of fields in the struct.
func (c *FieldSetter) NumFields() int {
	return c.val.NumField()
}

// RequestTests defines test cases for the TestRequest and
// TestCommentValues functions.
var RequestTests = []struct {
	// Name of a field in the Comment struct.
	Field string
	// Value to be assigned to the field.
	Value interface{}
	// Expected return value of Comment.Values given
	// a Comment with its fields set as above.
	Values map[string]string
	// Expected query string generated by the Checker
	// methods given the key-value pairs above.
	QueryString string
}{
	{
		// NOTE: Query strings should include the verified website
		// by default.
		QueryString: "blog=http%3A%2F%2Fexample.com",
	},
	{
		Field: "UserIP",
		Value: "127.0.0.1",
		Values: map[string]string{
			"user_ip": "127.0.0.1",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&user_ip=127.0.0.1",
	},
	{
		Field: "UserAgent",
		Value: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
		Values: map[string]string{
			"user_ip":    "127.0.0.1",
			"user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Referer",
		Value: "http://www.google.com",
		Values: map[string]string{
			"user_ip":    "127.0.0.1",
			"user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":   "http://www.google.com",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Page",
		Value: "http://example.com/posts/this-is-a-post/",
		Values: map[string]string{
			"user_ip":    "127.0.0.1",
			"user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":   "http://www.google.com",
			"permalink":  "http://example.com/posts/this-is-a-post/",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "PageTimestamp",
		// NOTE: Timestamps should be converted to UTC time.
		Value: time.Date(2016, time.March, 31, 18, 27, 59, 0, UTCMinus5),
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Type",
		Value: "comment",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Author",
		Value: "Funny commenter name",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_author=Funny+commenter+name&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "AuthorEmail",
		Value: "first.last@gmail.com",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "AuthorSite",
		Value: "http://blog.domain.com",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
			"comment_author_url":        "http://blog.domain.com",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Content",
		Value: "<p>This blog comment contains <strong>bold</strong> and <em>italic</em> text.</p>",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
			"comment_author_url":        "http://blog.domain.com",
			"comment_content":           "<p>This blog comment contains <strong>bold</strong> and <em>italic</em> text.</p>",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Timestamp",
		// NOTE: Timestamps should be converted to UTC time.
		Value: time.Date(2016, time.April, 1, 9, 0, 0, 0, UTCMinus5),
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
			"comment_author_url":        "http://blog.domain.com",
			"comment_content":           "<p>This blog comment contains <strong>bold</strong> and <em>italic</em> text.</p>",
			"comment_date_gmt":          "2016-04-01T14:00:00Z",
		},
		QueryString: "blog=http%3A%2F%2Fexample.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Site",
		// NOTE: Websites specified in the comment data should override the default website.
		Value: "http://anothersite.com",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
			"comment_author_url":        "http://blog.domain.com",
			"comment_content":           "<p>This blog comment contains <strong>bold</strong> and <em>italic</em> text.</p>",
			"comment_date_gmt":          "2016-04-01T14:00:00Z",
			"blog":                      "http://anothersite.com",
		},
		QueryString: "blog=http%3A%2F%2Fanothersite.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "SiteLanguage",
		Value: "en_us",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
			"comment_author_url":        "http://blog.domain.com",
			"comment_content":           "<p>This blog comment contains <strong>bold</strong> and <em>italic</em> text.</p>",
			"comment_date_gmt":          "2016-04-01T14:00:00Z",
			"blog":                      "http://anothersite.com",
			"blog_lang":                 "en_us",
		},
		QueryString: "blog=http%3A%2F%2Fanothersite.com&blog_lang=en_us&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "SiteCharset",
		Value: "UTF-8",
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
			"comment_type":              "comment",
			"comment_author":            "Funny commenter name",
			"comment_author_email":      "first.last@gmail.com",
			"comment_author_url":        "http://blog.domain.com",
			"comment_content":           "<p>This blog comment contains <strong>bold</strong> and <em>italic</em> text.</p>",
			"comment_date_gmt":          "2016-04-01T14:00:00Z",
			"blog":                      "http://anothersite.com",
			"blog_lang":                 "en_us",
			"blog_charset":              "UTF-8",
		},
		QueryString: "blog=http%3A%2F%2Fanothersite.com&blog_charset=UTF-8&blog_lang=en_us&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
}

// TestNewCheckers verifies that NewChecker and NewCheckerClient
// work as expected.
func TestNewCheckers(t *testing.T) {

	// The following calls should be functionally equivalent.
	ch1 := gokismet.NewChecker(TestAPIKey, TestSite)
	ch2 := gokismet.NewCheckerClient(TestAPIKey, TestSite, nil)
	ch3 := gokismet.NewCheckerClient(TestAPIKey, TestSite, http.DefaultClient)

	if ch1 == ch2 || ch1 == ch3 || ch2 == ch3 {
		t.Errorf("NewChecker[Client] should create distinct Checkers")
	}

	if *ch1 != *ch3 {
		t.Errorf("Calling NewChecker should be the same as calling NewCheckerClient with the default client")
	}

	if *ch2 != *ch3 {
		t.Errorf("Calling NewCheckerClient with a nil Client should be the same as calling it with the default client")
	}
}

// TestCommentValues verifies that Comment.Values generates the
// correct key-value pairs.
func TestCommentValues(t *testing.T) {

	comment := &gokismet.Comment{}
	setter := NewFieldSetter(comment)

	// RequestTests should have one test case for each Comment
	// field plus an extra test case for when no fields are set.
	if len(RequestTests) != setter.NumFields()+1 {
		t.Errorf("Not all Comment fields are being tested: expected %d fields, got %d",
			setter.NumFields()+1, len(RequestTests))
	}

	// Set the Comment fields one at a time and check that
	// Comment.Values returns the expected key-value pairs.
	for i, test := range RequestTests {

		if test.Field != "" {
			if err := setter.Set(test.Field, test.Value); err != nil {
				t.Errorf("Test %d: %s", i+1, err)
				continue
			}
		}

		errors := compareStringMaps(test.Values, comment.Values(), "Key-Value pair(s)")
		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}

// TestRequest_Check verifies that Checker.Check produces
// well-formed HTTP requests.
func TestRequest_Check(t *testing.T) {

	fn := toErrorFunc(fnCheck)
	url := "https://123456789abc.rest.akismet.com/1.1/comment-check"

	testRequest(t, fn, url)
}

// TestRequest_ReportHam verifies that Checker.ReportHam produces
// well-formed HTTP requests.
func TestRequest_ReportHam(t *testing.T) {

	fn := fnReportHam
	url := "https://123456789abc.rest.akismet.com/1.1/submit-ham"

	testRequest(t, fn, url)
}

// TestRequest_ReportSpam verifies that Checker.ReportSpam produces
// well-formed HTTP requests.
func TestRequest_ReportSpam(t *testing.T) {

	fn := fnReportSpam
	url := "https://123456789abc.rest.akismet.com/1.1/submit-spam"

	testRequest(t, fn, url)
}

// testRequest verifies that HTTP requests for the given Checker
// method are well-formed.
func testRequest(t *testing.T, fn ErrorFunc, expectedURL string) {

	client := &RequestStore{}
	verifyingClient := adaptClient(client, withResponder(verifyingResponder))

	ch := gokismet.NewCheckerClient(TestAPIKey, TestSite, verifyingClient)

	// For each test case in the request test data, call the
	// Checker method with the test's key-value pairs and
	// validate the resulting request.
	for i, test := range RequestTests {

		fn(ch, test.Values)

		// The first method call generates two requests: one to
		// verify the API key and one to actually make the call.
		// All subsequent method calls add one request to the
		// RequestStore. So when i == 0 we should have 2 requests
		// in the RequestStore. When i == 1 we should have 3, and
		// so on...
		if len(client.Requests) != i+2 {
			t.Fatalf("Test %d: Expected %d request(s), got %d", i+1, i+2, len(client.Requests))
		}

		var errors []error

		// Check the verify key request from the first method call.
		if i == 0 {
			exp := requestInfoFor("https://rest.akismet.com/1.1/verify-key", "blog=http%3A%2F%2Fexample.com&key=123456789abc")
			errors = append(errors, compareRequestInfo(exp, client.Requests[0])...)
		}

		// Check the request from the most recent method call.
		exp := requestInfoFor(expectedURL, test.QueryString)
		errors = append(errors, compareRequestInfo(exp, client.Requests[i+1])...)

		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}

// requestInfoFor returns the expected request data for a given
// Akismet endpoint URL and query string.
func requestInfoFor(url, body string) *RequestInfo {
	return &RequestInfo{
		Method: "POST",
		URL:    url,
		HeaderItems: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
			"User-Agent":   "Gokismet/3.0",
		},
		Body: body,
	}
}

// A ResponseTest defines a test case for the TestResponse functions.
type ResponseTest struct {
	// Does this test need a verified API key?
	IsVerified bool
	// Mock Akismet responses to feed into gokismet.
	Responses map[string]*ResponseInfo
	// The expected spam status given the responses above.
	SpamStatus gokismet.SpamStatus
	// The expected error given the responses above.
	Error error
}

// sharedResponseTests returns the ResponseTest cases that are common
// to all three Checker methods.
func sharedResponseTests(method string) []ResponseTest {
	return []ResponseTest{
		{
			// Error while verifying the API key.
			Error: errors.New(`No response for "verify-key"`),
		},
		{
			// Non-200 HTTP status while verifying the API key.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					StatusCode: http.StatusMovedPermanently,
				},
			},
			Error: errors.New("got 301 Moved Permanently from https://rest.akismet.com/1.1/verify-key"),
		},
		{
			// API key not verified.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:       "invalid",
					StatusCode: http.StatusOK,
				},
			},
			Error: &gokismet.KeyError{
				Key:  "123456789abc",
				Site: "http://example.com",
				ValError: &gokismet.ValError{
					Method:   "verify-key",
					Response: "invalid",
				},
			},
		},
		{
			// API key not verified with help message.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:       "invalid",
					StatusCode: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-debug-help": "A helpful diagnostic message",
					},
				},
			},
			Error: &gokismet.KeyError{
				Key:  "123456789abc",
				Site: "http://example.com",
				ValError: &gokismet.ValError{
					Method:   "verify-key",
					Response: "invalid",
					Hint:     "A helpful diagnostic message",
				},
			},
		},
		{
			// Error during Akismet call.
			IsVerified: true,
			Error:      errors.New("No response for \"" + method + "\""),
		},
		{
			// Non-200 HTTP status from Akismet call.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				method: {
					StatusCode: http.StatusInternalServerError,
				},
			},
			Error: errors.New("got 500 Internal Server Error from https://123456789abc.rest.akismet.com/1.1/" + method),
		},
		{
			// Unexpected return value from Akismet call.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				method: {
					Body:       "invalid",
					StatusCode: http.StatusOK,
				},
			},
			Error: &gokismet.ValError{
				Method:   method,
				Response: "invalid",
			},
		},
		{
			// Unexpected return value with help message.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				method: {
					Body:       "invalid",
					StatusCode: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-debug-help": "A helpful diagnostic message",
					},
				},
			},
			Error: &gokismet.ValError{
				Method:   method,
				Response: "invalid",
				Hint:     "A helpful diagnostic message",
			},
		},
	}
}

// TestResponse_Check verifies that Checker.Check returns the
// correct values for various Akismet responses.
func TestResponse_Check(t *testing.T) {

	checkTests := []ResponseTest{
		{
			// Negative spam response.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				"comment-check": {
					Body:       "false",
					StatusCode: http.StatusOK,
				},
			},
			SpamStatus: gokismet.StatusHam,
		},
		{
			// Positive spam response.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				"comment-check": {
					Body:       "true",
					StatusCode: http.StatusOK,
				},
			},
			SpamStatus: gokismet.StatusProbableSpam,
		},
		{
			// Pervasive spam response.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				"comment-check": {
					Body:       "true",
					StatusCode: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-pro-tip": "discard",
					},
				},
			},
			SpamStatus: gokismet.StatusDefiniteSpam,
		},
	}

	tests := sharedResponseTests("comment-check")
	tests = append(tests, checkTests...)

	testResponse(t, fnCheck, tests)
}

// TestResponse_ReportHam verifies that Checker.ReportHam
// returns the correct values for various Akismet responses.
func TestResponse_ReportHam(t *testing.T) {
	testResponse_Report(t, "submit-ham", fnReportHam)
}

// TestResponse_ReportSpam verifies that Checker.ReportSpam
// returns the correct values for various Akismet responses.
func TestResponse_ReportSpam(t *testing.T) {
	testResponse_Report(t, "submit-spam", fnReportSpam)
}

// testResponse_Report handles the heavy lifting for the
// TestResponse_ReportHam and TestResponse_ReportSpam functions.
func testResponse_Report(t *testing.T, method string, submit ErrorFunc) {

	reportTests := []ResponseTest{
		{
			// Success response.
			IsVerified: true,
			Responses: map[string]*ResponseInfo{
				method: {
					Body:       "Thanks for making the web a better place.",
					StatusCode: http.StatusOK,
				},
			},
		},
	}

	tests := sharedResponseTests(method)
	tests = append(tests, reportTests...)

	testResponse(t, toStatusErrorFunc(submit), tests)
}

// testResponse verifies that a Checker method returns the
// correct values for the Akismet responses provided.
func testResponse(t *testing.T, fn StatusErrorFunc, tests []ResponseTest) {

	for i, test := range tests {

		client := &Responder{
			Responses: test.Responses,
		}
		if test.IsVerified {
			client.AddResponses(verifyingResponder)
		}

		ch := gokismet.NewCheckerClient(TestAPIKey, TestSite, client)
		status, err := fn(ch, nil)

		if status != test.SpamStatus {
			t.Errorf("Test %d: Expected Spam Status %q, got %q", i+1,
				statusToString(test.SpamStatus), statusToString(status))
		}

		errors := compareError(test.Error, err)
		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}

// TestError_ValError tests string formatting for the ValError
// type.
func TestError_ValError(t *testing.T) {

	tests := []struct {
		Method   string
		Response string
		Hint     string
		Expected string
	}{
		{
			Method:   "comment-check",
			Expected: `comment-check returned an empty string (expected true or false)`,
		},
		{
			Method:   "comment-check",
			Hint:     "A helpful diagnostic message",
			Expected: `comment-check returned an empty string (A helpful diagnostic message)`,
		},
		{
			Method:   "comment-check",
			Response: "invalid",
			Expected: `comment-check returned "invalid" (expected true or false)`,
		},
		{
			Method:   "comment-check",
			Response: "invalid",
			Hint:     "A helpful diagnostic message",
			Expected: `comment-check returned "invalid" (A helpful diagnostic message)`,
		},
		{
			Method:   "submit-ham",
			Expected: `submit-ham returned an empty string (expected thank you message)`,
		},
		{
			Method:   "submit-ham",
			Hint:     "A helpful diagnostic message",
			Expected: `submit-ham returned an empty string (A helpful diagnostic message)`,
		},
		{
			Method:   "submit-ham",
			Response: "invalid",
			Expected: `submit-ham returned "invalid" (expected thank you message)`,
		},
		{
			Method:   "submit-ham",
			Response: "invalid",
			Hint:     "A helpful diagnostic message",
			Expected: `submit-ham returned "invalid" (A helpful diagnostic message)`,
		},
		{
			Method:   "submit-spam",
			Expected: `submit-spam returned an empty string (expected thank you message)`,
		},
		{
			Method:   "submit-spam",
			Hint:     "A helpful diagnostic message",
			Expected: `submit-spam returned an empty string (A helpful diagnostic message)`,
		},
		{
			Method:   "submit-spam",
			Response: "invalid",
			Expected: `submit-spam returned "invalid" (expected thank you message)`,
		},
		{
			Method:   "submit-spam",
			Response: "invalid",
			Hint:     "A helpful diagnostic message",
			Expected: `submit-spam returned "invalid" (A helpful diagnostic message)`,
		},
	}

	for i, test := range tests {

		err := gokismet.ValError{
			Method:   test.Method,
			Response: test.Response,
			Hint:     test.Hint,
		}

		if got := err.Error(); got != test.Expected {
			t.Errorf("Test %d: Expected %q, got %q", i+1, test.Expected, got)
		}
	}
}

// TestError_KeyError tests string formatting for the KeyError
// type.
func TestError_KeyError(t *testing.T) {

	tests := []struct {
		Response string
		Hint     string
		Expected string
	}{
		{
			Expected: `key 123456789abc not verified: verify-key returned an empty string`,
		},
		{
			Hint:     "A helpful diagnostic message",
			Expected: `key 123456789abc not verified: verify-key returned an empty string (A helpful diagnostic message)`,
		},
		{
			Response: "invalid",
			Expected: `key 123456789abc not verified: verify-key returned "invalid"`,
		},
		{
			Response: "invalid",
			Hint:     "A helpful diagnostic message",
			Expected: `key 123456789abc not verified: verify-key returned "invalid" (A helpful diagnostic message)`,
		},
	}

	for i, test := range tests {

		err := gokismet.KeyError{
			Key:  TestAPIKey,
			Site: TestSite,
			ValError: &gokismet.ValError{
				Method:   "verify-key",
				Response: test.Response,
				Hint:     test.Hint,
			},
		}

		if got := err.Error(); got != test.Expected {
			t.Errorf("Test %d: Expected %q, got %q", i+1, test.Expected, got)
		}
	}
}

// An AkismetTest defines a test case for the TestAkismet
// functions.
type AkismetTest struct {
	// Additional comment parameters to pass to Akismet.
	Params map[string]string
	// Expected spam status returned by gokismet.
	SpamStatus gokismet.SpamStatus
	// Expected error returned by gokismet.
	Error error
}

// TestAkismet_Check uses the live Akismet API to validate
// the results of the Checker.Check method.
func TestAkismet_Check(t *testing.T) {

	tests := []AkismetTest{
		{
			// A user_role of "administrator" should trigger
			// a negative spam response from Akismet.
			Params: map[string]string{
				"is_test":   "true",
				"user_role": "administrator",
			},
			SpamStatus: gokismet.StatusHam,
		},
		{
			// A comment_author of "viagra-test-123" should
			// trigger a positive spam response from Akismet.
			Params: map[string]string{
				"is_test":        "true",
				"comment_author": "viagra-test-123",
			},
			SpamStatus: gokismet.StatusProbableSpam,
		},
		{
			// Adding test_discard should trigger a "pervasive"
			// spam response from Akismet.
			Params: map[string]string{
				"is_test":        "true",
				"comment_author": "viagra-test-123",
				"test_discard":   "true",
			},
			SpamStatus: gokismet.StatusDefiniteSpam,
		},
	}

	testAkismet(t, fnCheck, tests)
}

// TestAkismet_ReportHam uses the live Akismet API to
// validate the results of the Checker.ReportHam method.
func TestAkismet_ReportHam(t *testing.T) {
	testAkismet_Report(t, fnReportHam)
}

// TestAkismet_ReportSpam uses the live Akismet API to
// validate the results of the Checker.ReportSpam method.
func TestAkismet_ReportSpam(t *testing.T) {
	testAkismet_Report(t, fnReportSpam)
}

// testAkismet_Report handles the heavy lifting for the
// TestAkismet_ReportSpam and TestAkismet_ReportHam functions.
func testAkismet_Report(t *testing.T, submit ErrorFunc) {

	tests := []AkismetTest{
		{
			// This should trigger a success response.
			Params: map[string]string{
				"is_test": "true",
			},
		},
	}

	testAkismet(t, toStatusErrorFunc(submit), tests)
}

// testAkismet uses the live Akismet API to verify that
// a Checker method returns the expected results.
func testAkismet(t *testing.T, fn StatusErrorFunc, tests []AkismetTest) {

	// Only run this test if we have an API key and website.
	if *flags.APIKey == "" || *flags.Site == "" {
		t.SkipNow()
	}

	// Values are from the Akismet API docs.
	comment := &gokismet.Comment{
		UserIP:      "127.0.0.1",
		UserAgent:   "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		Referer:     "http://www.google.com",
		Page:        path.Join(*flags.Site, "blog/post=1"),
		Type:        "comment",
		Author:      "admin",
		AuthorEmail: "test@test.com",
		AuthorSite:  "http://www.CheckOutMyCoolSite.com",
		Content:     "It means a lot that you would take the time to review our software. Thanks again.",
	}

	ch := gokismet.NewChecker(*flags.APIKey, *flags.Site)

	for i, test := range tests {

		values := comment.Values()
		// Add any custom parameters to our comment data.
		for k, v := range test.Params {
			values[k] = v
		}

		// Call the Checker method.
		status, err := fn(ch, values)

		// Check the returned spam status and error.
		if status != test.SpamStatus {
			t.Errorf("Test %d: Expected Spam Status %q, got %q", i+1,
				statusToString(test.SpamStatus), statusToString(status))
		}

		errors := compareError(test.Error, err)
		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}

// statusToString returns a string representation of
// a SpamStatus.
func statusToString(status gokismet.SpamStatus) string {
	switch status {
	case gokismet.StatusUnknown:
		return "Unknown"
	case gokismet.StatusHam:
		return "Ham"
	case gokismet.StatusProbableSpam:
		return "Probable Spam"
	case gokismet.StatusDefiniteSpam:
		return "Definite Spam"
	}

	panic("statusToString: unknown status")
}

//
// Here follows a bunch of comparison functions used to verify
// that two instances of a particular type have the same values.
//

func compareStringMaps(exp, got map[string]string, things string) []error {

	var errors []error

	if len(got) != len(exp) {
		return append(errors, fmt.Errorf("Expected %d %s, got %d", len(exp), things, len(got)))
	}

	for k, vexp := range exp {
		if vgot := got[k]; vgot != vexp {
			errors = append(errors, fmt.Errorf("Expected %s %q, got %q", k, vexp, vgot))
		}
	}

	return errors
}

func compareRequestInfo(exp, got *RequestInfo) []error {

	var errors []error

	if got.Method != exp.Method {
		errors = append(errors, fmt.Errorf("Expected Request Method %q, got %q", exp.Method, got.Method))
	}

	if got.URL != exp.URL {
		errors = append(errors, fmt.Errorf("Expected Request URL %q, got %q", exp.URL, got.URL))
	}

	errors = append(errors, compareStringMaps(exp.HeaderItems, got.HeaderItems, "Request Header(s)")...)

	if got.Body != exp.Body {
		errors = append(errors, fmt.Errorf("Expected Request Body %q, got %q", exp.Body, got.Body))
	}

	return errors
}

func compareError(exp, got error) []error {

	switch exp := exp.(type) {
	case nil:
		if got != nil {
			return []error{
				fmt.Errorf("Expected nil error, got %T %s", got, got),
			}
		}
		return nil
	case *gokismet.KeyError:
		err, ok := got.(*gokismet.KeyError)
		if !ok {
			return []error{
				fmt.Errorf("Expected a KeyError, got %T %s", got, got),
			}
		}
		return compareKeyError(exp, err)
	case *gokismet.ValError:
		err, ok := got.(*gokismet.ValError)
		if !ok {
			return []error{
				fmt.Errorf("Expected a ValError, got %T %s", got, got),
			}
		}
		return compareValError(exp, err)
	default:
		if got.Error() != exp.Error() {
			return []error{
				fmt.Errorf("Expected error %v, got %v", exp, got),
			}
		}
		return nil
	}
}

func compareValError(exp, got *gokismet.ValError) []error {

	var errors []error

	if got.Method != exp.Method {
		errors = append(errors, fmt.Errorf("Expected a ValError with Method %q, got %q", exp.Method, got.Method))
	}

	if got.Response != exp.Response {
		errors = append(errors, fmt.Errorf("Expected a ValError with Response %q, got %q", exp.Response, got.Response))
	}

	if got.Hint != exp.Hint {
		errors = append(errors, fmt.Errorf("Expected a ValError with Hint %q, got %q", exp.Hint, got.Hint))
	}

	return errors
}

func compareKeyError(exp, got *gokismet.KeyError) []error {

	var errors []error

	if got.Key != exp.Key {
		errors = append(errors, fmt.Errorf("Expected a KeyError with Key %q, got %q", exp.Key, got.Key))
	}

	if got.Site != exp.Site {
		errors = append(errors, fmt.Errorf("Expected a KeyError with Site %q, got %q", exp.Site, got.Site))
	}

	if got.Method != exp.Method {
		errors = append(errors, fmt.Errorf("Expected a KeyError with Method %q, got %q", exp.Method, got.Method))
	}

	if got.Response != exp.Response {
		errors = append(errors, fmt.Errorf("Expected a KeyError with Response %q, got %q", exp.Response, got.Response))
	}

	if got.Hint != exp.Hint {
		errors = append(errors, fmt.Errorf("Expected a KeyError with Hint %q, got %q", exp.Hint, got.Hint))
	}

	return errors
}
