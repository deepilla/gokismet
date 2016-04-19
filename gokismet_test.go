package gokismet_test

import (
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

const (
	TESTAPIKEY = "123456789abc"
	TESTSITE   = "http://www.example.com"
)

var UTCMinus5 = time.FixedZone("UTCMinus5", -5*60*60)

var (
	fnSubmitHam    = (*gokismet.API).SubmitHam
	fnSubmitSpam   = (*gokismet.API).SubmitSpam
	fnCheckComment = (*gokismet.API).CheckComment
)

type (
	ErrorFunc       func(*gokismet.API, map[string]string) error
	StatusErrorFunc func(*gokismet.API, map[string]string) (gokismet.SpamStatus, error)
)

func toStatusErrorFunc(fn ErrorFunc) StatusErrorFunc {
	return func(api *gokismet.API, values map[string]string) (gokismet.SpamStatus, error) {
		return gokismet.StatusUnknown, fn(api, values)
	}
}

func toErrorFunc(fn StatusErrorFunc) ErrorFunc {
	return func(api *gokismet.API, values map[string]string) error {
		_, err := fn(api, values)
		return err
	}
}

// RequestInfo contains the pertinent parts of an
// HTTP Request.
type RequestInfo struct {
	Method      string
	URL         string
	HeaderItems map[string]string
	Body        string
}

func NewRequestInfo(req *http.Request) *RequestInfo {

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
			// Swallow this error but write it to the
			// RequestInfo body so that we can see what
			// happened when the tests fail.
			info.Body = "ReadAll Error: " + err.Error()
		} else {
			info.Body = string(body)
		}
	}

	return info
}

// ResponseInfo contains the pertinent parts of an
// HTTP Response.
type ResponseInfo struct {
	Status      int
	HeaderItems map[string]string
	Body        string
}

// MakeResponse creates and returns a barebones HTTP
// Response object.
func (info *ResponseInfo) MakeResponse() *http.Response {

	resp := &http.Response{
		StatusCode: info.Status,
		Status:     fmt.Sprintf("%d %s", info.Status, http.StatusText(info.Status)),
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

// TestClient satisfies the gokismet Client interface.
// Its Do method captures incoming HTTP Requests and
// returns HTTP Responses defined by the caller. There
// are no actual HTTP requests.
type TestClient struct {
	// Incoming requests.
	Requests []*RequestInfo
	// Predefined responses keyed by command, e.g.
	// "verify-key". Set when the TestClient is created.
	Responses map[string]*ResponseInfo
}

// NewVerifyingTestClient returns a TestClient that
// verifies API keys.
func NewVerifyingTestClient() *TestClient {
	return &TestClient{
		Responses: map[string]*ResponseInfo{
			"verify-key": {
				Body:   "valid",
				Status: http.StatusOK,
			},
		},
	}
}

// Do adds the request info to the array and returns
// the predefined response for this request type.
func (c *TestClient) Do(req *http.Request) (*http.Response, error) {

	info := NewRequestInfo(req)
	c.Requests = append(c.Requests, info)

	return c.respond(info.URL)
}

// ResetRequests clears any existing HTTP request info.
func (c *TestClient) ResetRequests() {
	c.Requests = c.Requests[:0]
}

// respond extracts the command from an Akismet REST URL
// and returns the corresponding HTTP response (as defined
// by the object creator).
func (c *TestClient) respond(curl string) (*http.Response, error) {

	// URL format is https://rest.akismet.com/1.1/verify-key.
	// The last part of the URL, as returned by the Base method,
	// is the actual command.
	cmd := path.Base(curl)

	if info := c.Responses[cmd]; info != nil {
		return info.MakeResponse(), nil
	}

	return nil, &TestClientError{
		Command: cmd,
	}
}

// A TestClientError is returned by TestClient's Do method
// when the current HTTP request does not have a predefined
// response.
type TestClientError struct {
	Command string
}

func (e *TestClientError) Error() string {
	return "Unexpected command " + e.Command
}

// A FieldSetter uses reflection to set the fields of a
// struct.
type FieldSetter struct {
	val reflect.Value
}

// NewFieldSetter takes a pointer to a struct and returns
// a FieldSetter that can set the fields of that struct.
func NewFieldSetter(val interface{}) *FieldSetter {
	return &FieldSetter{reflect.Indirect(reflect.ValueOf(val))}
}

// Set sets the field with the given name to the given value.
func (c *FieldSetter) Set(name string, val interface{}) error {

	// Panics if val is not a struct. Returns a nil Value
	// if the field does not exist.
	v := c.val.FieldByName(name)

	switch {
	case !v.IsValid():
		return fmt.Errorf("Field %s not found", name)
	case !v.CanSet():
		return fmt.Errorf("Field %s is not settable", name)
	default:
		v.Set(reflect.ValueOf(val))
		return nil
	}
}

// NumFields returns the number of fields in the struct.
func (c *FieldSetter) NumFields() int {
	return c.val.NumField()
}

// apiCallToString returns a string representation of an
// APICall enum value.
func apiCallToString(call gokismet.APICall) string {
	switch call {
	case gokismet.APIVerifyKey:
		return "VerifyKey"
	case gokismet.APICheckComment:
		return "CheckComment"
	case gokismet.APISubmitHam:
		return "SubmitHam"
	case gokismet.APISubmitSpam:
		return "SubmitSpam"
	default:
		return "!Invalid API Call!"
	}
}

// spamStatusToString returns a string representation of a
// SpamStatus enum value.
func spamStatusToString(status gokismet.SpamStatus) string {
	switch status {
	case gokismet.StatusUnknown:
		return "Unknown"
	case gokismet.StatusHam:
		return "Not Spam"
	case gokismet.StatusProbableSpam:
		return "Probable Spam"
	case gokismet.StatusDefiniteSpam:
		return "Definite Spam"
	default:
		return "!Invalid Status!"
	}
}

//
// Here follows a bunch of compareXXX functions used to verify
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

func compareKeyValuePairs(exp, got map[string]string) []error {
	return compareStringMaps(exp, got, "Key-Value pair(s)")
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
	case *TestClientError:
		if got, ok := got.(*TestClientError); ok {
			return compareTestClientError(exp, got)
		}
	case *gokismet.AuthError:
		if got, ok := got.(*gokismet.AuthError); ok {
			return compareAuthError(exp, got)
		}
	case *gokismet.APIError:
		if got, ok := got.(*gokismet.APIError); ok {
			return compareAPIError(exp, got)
		}
	default:
		return []error{
			fmt.Errorf("Cannot handle 'expected' error %T %s", exp, exp),
		}
	}

	return []error{
		fmt.Errorf("Expected error %T %s, got %T %s", exp, exp, got, got),
	}
}

func compareTestClientError(exp, got *TestClientError) []error {

	var errors []error

	if got.Command != exp.Command {
		errors = append(errors, fmt.Errorf("Expected a TestClientError with Command %q, got %q", exp.Command, got.Command))
	}

	return errors
}

func compareAPIError(exp, got *gokismet.APIError) []error {

	var errors []error

	if got.Call != exp.Call {
		errors = append(errors, fmt.Errorf("Expected an APIError with Call %q, got %q",
			apiCallToString(exp.Call), apiCallToString(got.Call)))
	}

	if got.Result != exp.Result {
		errors = append(errors, fmt.Errorf("Expected an APIError with Result %q, got %q", exp.Result, got.Result))
	}

	if got.Help != exp.Help {
		errors = append(errors, fmt.Errorf("Expected an APIError with Help Text %q, got %q", exp.Help, got.Help))
	}

	return errors
}

func compareAuthError(exp, got *gokismet.AuthError) []error {

	var errors []error

	if got.Key != exp.Key {
		errors = append(errors, fmt.Errorf("Expected an AuthError with Key %q, got %q", exp.Key, got.Key))
	}

	if got.Site != exp.Site {
		errors = append(errors, fmt.Errorf("Expected an AuthError with Site %q, got %q", exp.Site, got.Site))
	}

	if got.Result != exp.Result {
		errors = append(errors, fmt.Errorf("Expected an AuthError with Result %q, got %q", exp.Result, got.Result))
	}

	if got.Help != exp.Help {
		errors = append(errors, fmt.Errorf("Expected an AuthError with Help Text %q, got %q", exp.Help, got.Help))
	}

	return errors
}

// Test data for the TestCommentValue and TestRequestXXX functions.
var CommentData = []struct {
	// Name of a field in the Comment type.
	Field string
	// Value to be assigned to the field.
	Value interface{}
	// Expected return value of Comment.Values given
	// a Comment with its fields set as above.
	Values map[string]string
	// Expected query string generated by the API type
	// given the key-value pairs above.
	EncodedValues string
}{
	{
		// NOTE: API should include the verified website by default.
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com",
	},
	{
		Field: "UserIP",
		Value: "127.0.0.1",
		Values: map[string]string{
			"user_ip": "127.0.0.1",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&user_ip=127.0.0.1",
	},
	{
		Field: "UserAgent",
		Value: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
		Values: map[string]string{
			"user_ip":    "127.0.0.1",
			"user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Referer",
		Value: "http://www.google.com",
		Values: map[string]string{
			"user_ip":    "127.0.0.1",
			"user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":   "http://www.google.com",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "PageTimestamp",
		// NOTE: Times should be converted to UTC.
		Value: time.Date(2016, time.March, 31, 18, 27, 59, 0, UTCMinus5),
		Values: map[string]string{
			"user_ip":                   "127.0.0.1",
			"user_agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36",
			"referrer":                  "http://www.google.com",
			"permalink":                 "http://example.com/posts/this-is-a-post/",
			"comment_post_modified_gmt": "2016-03-31T23:27:59Z",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_author=Funny+commenter+name&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "AuthorPage",
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Timestamp",
		// NOTE: Times should be converted to UTC.
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
		EncodedValues: "blog=http%3A%2F%2Fwww.example.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
	{
		Field: "Site",
		// NOTE: This should override the default website.
		Value: "http://www.anothersite.com",
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
			"blog":                      "http://www.anothersite.com",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.anothersite.com&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
			"blog":                      "http://www.anothersite.com",
			"blog_lang":                 "en_us",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.anothersite.com&blog_lang=en_us&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
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
			"blog":                      "http://www.anothersite.com",
			"blog_lang":                 "en_us",
			"blog_charset":              "UTF-8",
		},
		EncodedValues: "blog=http%3A%2F%2Fwww.anothersite.com&blog_charset=UTF-8&blog_lang=en_us&comment_author=Funny+commenter+name&comment_author_email=first.last%40gmail.com&comment_author_url=http%3A%2F%2Fblog.domain.com&comment_content=%3Cp%3EThis+blog+comment+contains+%3Cstrong%3Ebold%3C%2Fstrong%3E+and+%3Cem%3Eitalic%3C%2Fem%3E+text.%3C%2Fp%3E&comment_date_gmt=2016-04-01T14%3A00%3A00Z&comment_post_modified_gmt=2016-03-31T23%3A27%3A59Z&comment_type=comment&permalink=http%3A%2F%2Fexample.com%2Fposts%2Fthis-is-a-post%2F&referrer=http%3A%2F%2Fwww.google.com&user_agent=Mozilla%2F5.0+%28X11%3B+Linux+x86_64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F41.0.2227.0+Safari%2F537.36&user_ip=127.0.0.1",
	},
}

// TestCommentValues tests the Comment.Values method.
func TestCommentValues(t *testing.T) {

	comment := &gokismet.Comment{}
	setter := NewFieldSetter(comment)

	if len(CommentData) != setter.NumFields()+1 {
		t.Errorf("Not all Comment fields are being tested: expected %d fields, got %d",
			setter.NumFields()+1, len(CommentData))
	}

	// Loop through the comment data, setting each field
	// individually and checking that Comment.Values
	// returns the expected key-value pairs.

	for i, test := range CommentData {

		if test.Field != "" {
			if err := setter.Set(test.Field, test.Value); err != nil {
				t.Errorf("Test %d: %s", i+1, err)
				continue
			}
		}

		errors := compareKeyValuePairs(test.Values, comment.Values())
		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}

// TestRequestCheckComment tests the API.CheckComment request headers.
func TestRequestCheckComment(t *testing.T) {

	fn := toErrorFunc(fnCheckComment)
	url := "https://123456789abc.rest.akismet.com/1.1/comment-check"

	testRequest(t, fn, url)
}

// TestRequestSubmitHam tests the API.SubmitHam request headers.
func TestRequestSubmitHam(t *testing.T) {

	fn := fnSubmitHam
	url := "https://123456789abc.rest.akismet.com/1.1/submit-ham"

	testRequest(t, fn, url)
}

// TestRequestSubmitSpam tests the API.SubmitSpam request headers.
func TestRequestSubmitSpam(t *testing.T) {

	fn := fnSubmitSpam
	url := "https://123456789abc.rest.akismet.com/1.1/submit-spam"

	testRequest(t, fn, url)
}

// testRequest is a general function for testing the request
// headers of API calls.
func testRequest(t *testing.T, fn ErrorFunc, url string) {

	// Our client is set up to verify API keys.
	client := NewVerifyingTestClient()

	// This function checks that we have the expected number
	// of client requests and that they are well-formed.
	doTest := func(testnum int, url string, body string, nReqs int, idx int) {

		if len(client.Requests) != nReqs {
			t.Fatalf("Test %d: Expected %d client request(s), got %d", testnum, nReqs, len(client.Requests))
		}

		exp := &RequestInfo{
			Method: "POST",
			URL:    url,
			HeaderItems: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"User-Agent":   "Gokismet/2.0",
			},
			Body: body,
		}

		errors := compareRequestInfo(exp, client.Requests[idx])
		for _, err := range errors {
			t.Errorf("Test %d: %s", testnum, err)
		}
	}

	api := gokismet.NewAPIWithClient(TESTAPIKEY, TESTSITE, client)

	// Execute the provided method.
	// This will be CheckComment, SubmitHam, or SubmitSpam.
	fn(api, nil)

	// The first API call should yield two client requests:
	// one to verify the key and one to make the actual API
	// call. Check the verify key request first.
	doTest(0, "https://rest.akismet.com/1.1/verify-key",
		"blog=http%3A%2F%2Fwww.example.com&key=123456789abc", 2, 0)

	// Then call the provided method again, once for each
	// item of comment data, and check those requests.
	for i, test := range CommentData {

		client.ResetRequests()
		fn(api, test.Values)

		doTest(i+1, url, test.EncodedValues, 1, 0)
	}
}

// A test data type for the TestResponseXXX functions.
type TestResponseData struct {
	Responses map[string]*ResponseInfo
	Status    gokismet.SpamStatus
	Error     error
}

// TestResponseCheckComment tests the status and error
// values returned by API.CheckComment.
func TestResponseCheckComment(t *testing.T) {

	// Test scenarios for a spam check.
	data := []TestResponseData{
		{
			// Invalid HTTP status.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				"comment-check": {
					Status: http.StatusInternalServerError,
				},
			},
			Error: &gokismet.APIError{
				Call:   gokismet.APICheckComment,
				Result: "Status 500 Internal Server Error",
			},
		},
		{
			// Negative spam response.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				"comment-check": {
					Body:   "false",
					Status: http.StatusOK,
				},
			},
			Status: gokismet.StatusHam,
		},
		{
			// Positive spam response.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				"comment-check": {
					Body:   "true",
					Status: http.StatusOK,
				},
			},
			Status: gokismet.StatusProbableSpam,
		},
		{
			// Pervasive spam response.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				"comment-check": {
					Body:   "true",
					Status: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-pro-tip": "discard",
					},
				},
			},
			Status: gokismet.StatusDefiniteSpam,
		},
		{
			// Error response.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				"comment-check": {
					Body:   "invalid",
					Status: http.StatusOK,
				},
			},
			Error: &gokismet.APIError{
				Call:   gokismet.APICheckComment,
				Result: "invalid",
			},
		},
		{
			// Error response with help message.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				"comment-check": {
					Body:   "invalid",
					Status: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-debug-help": "A helpful diagnostic message",
					},
				},
			},
			Error: &gokismet.APIError{
				Call:   gokismet.APICheckComment,
				Result: "invalid",
				Help:   "A helpful diagnostic message",
			},
		},
	}

	testResponse(t, fnCheckComment, data)
}

// TestResponseSubmitHam tests the error values returned
// by API.SubmitHam.
func TestResponseSubmitHam(t *testing.T) {

	call := gokismet.APISubmitHam
	submit := fnSubmitHam
	command := "submit-ham"

	testResponseSubmit(t, call, command, submit)
}

// TestResponseSubmitSpam tests the error values returned
// by API.SubmitSpam.
func TestResponseSubmitSpam(t *testing.T) {

	call := gokismet.APISubmitSpam
	submit := fnSubmitSpam
	command := "submit-spam"

	testResponseSubmit(t, call, command, submit)
}

// testResponseSubmit is a helper for TestResponseSubmitHam
// and TestResponseSubmitSpam.
func testResponseSubmit(t *testing.T, call gokismet.APICall, command string, submit ErrorFunc) {

	// Test scenarios for the submit API calls.
	data := []TestResponseData{
		{
			// Invalid HTTP status.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				command: {
					Status: http.StatusInternalServerError,
				},
			},
			Error: &gokismet.APIError{
				Call:   call,
				Result: "Status 500 Internal Server Error",
			},
		},
		{
			// Success response.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				command: {
					Body:   "Thanks for making the web a better place.",
					Status: http.StatusOK,
				},
			},
		},
		{
			// Error response.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				command: {
					Body:   "invalid",
					Status: http.StatusOK,
				},
			},
			Error: &gokismet.APIError{
				Call:   call,
				Result: "invalid",
			},
		},
		{
			// Error response with help message.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "valid",
					Status: http.StatusOK,
				},
				command: {
					Body:   "invalid",
					Status: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-debug-help": "A helpful diagnostic message",
					},
				},
			},
			Error: &gokismet.APIError{
				Call:   call,
				Result: "invalid",
				Help:   "A helpful diagnostic message",
			},
		},
	}

	testResponse(t, toStatusErrorFunc(submit), data)
}

// testResponse is a general function for testing the statuses
// and errors returned by API calls.
func testResponse(t *testing.T, fn StatusErrorFunc, moredata []TestResponseData) {

	// Test scenarios for all API calls.
	data := []TestResponseData{
		{
			// Error during key verification.
			Error: &TestClientError{
				Command: "verify-key",
			},
		},
		{
			// Key not verified.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "invalid",
					Status: http.StatusOK,
				},
			},
			Error: &gokismet.AuthError{
				Key:    "123456789abc",
				Site:   "http://www.example.com",
				Result: "invalid",
			},
		},
		{
			// Key not verified with help message.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Body:   "invalid",
					Status: http.StatusOK,
					HeaderItems: map[string]string{
						"X-akismet-debug-help": "A helpful diagnostic message",
					},
				},
			},
			Error: &gokismet.AuthError{
				Key:    "123456789abc",
				Site:   "http://www.example.com",
				Result: "invalid",
				Help:   "A helpful diagnostic message",
			},
		},
		{
			// Invalid HTTP status.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Status: http.StatusMovedPermanently,
				},
			},
			Error: &gokismet.APIError{
				Call:   gokismet.APIVerifyKey,
				Result: "Status 301 Moved Permanently",
			},
		},
		{
			// Invalid HTTP status with help message.
			Responses: map[string]*ResponseInfo{
				"verify-key": {
					Status: http.StatusNotFound,
					HeaderItems: map[string]string{
						"X-akismet-debug-help": "A helpful diagnostic message",
					},
				},
			},
			Error: &gokismet.APIError{
				Call:   gokismet.APIVerifyKey,
				Result: "Status 404 Not Found",
				Help:   "A helpful diagnostic message",
			},
		},
	}

	// Add method-specific test scenarios.
	data = append(data, moredata...)

	for i, test := range data {

		client := &TestClient{
			Responses: test.Responses,
		}

		api := gokismet.NewAPIWithClient(TESTAPIKEY, TESTSITE, client)
		status, err := fn(api, nil)

		if status != test.Status {
			t.Errorf("Test %d: Expected Spam Status %q, got %q", i+1,
				spamStatusToString(test.Status), spamStatusToString(status))
		}

		errors := compareError(test.Error, err)
		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}

// TestWrapClientCheckComment tests custom HTTP headers
// in API.CheckComment calls.
func TestWrapClientCheckComment(t *testing.T) {
	testWrapClient(t, toErrorFunc(fnCheckComment))
}

// TestWrapClientSubmitHam tests custom HTTP headers
// in API.SubmitHam calls.
func TestWrapClientSubmitHam(t *testing.T) {
	testWrapClient(t, fnSubmitHam)
}

// TestWrapClientSubmitSpam tests custom HTTP headers
// in API.SubmitSpam calls.
func TestWrapClientSubmitSpam(t *testing.T) {
	testWrapClient(t, fnSubmitSpam)
}

// testWrapClient tests that WrapClient injects the
// expected custom headers into HTTP requests.
func testWrapClient(t *testing.T, fn ErrorFunc) {

	data := []struct {
		Wrappers []map[string]string
		Expected map[string]string
	}{
		{
			// Default headers.
			Expected: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"User-Agent":   "Gokismet/2.0",
			},
		},
		{
			// Single wrapper.
			Wrappers: []map[string]string{
				map[string]string{
					"From":          "first.last@gmail.com",
					"Cache-Control": "no-cache",
				},
			},
			Expected: map[string]string{
				"Content-Type":  "application/x-www-form-urlencoded",
				"User-Agent":    "Gokismet/2.0",
				"From":          "first.last@gmail.com",
				"Cache-Control": "no-cache",
			},
		},
		{
			// Multiple wrappers.
			Wrappers: []map[string]string{
				map[string]string{
					"From": "first.last@gmail.com",
				},
				map[string]string{
					"Cache-Control": "no-cache",
				},
			},
			Expected: map[string]string{
				"Content-Type":  "application/x-www-form-urlencoded",
				"User-Agent":    "Gokismet/2.0",
				"From":          "first.last@gmail.com",
				"Cache-Control": "no-cache",
			},
		},
		{
			// Overwrite defaults.
			Wrappers: []map[string]string{
				map[string]string{
					"From": "first.last@gmail.com",
				},
				map[string]string{
					"Cache-Control": "no-cache",
				},
				map[string]string{
					"User-Agent": "GokismetTest/1.0 | " + gokismet.UA,
				},
			},
			Expected: map[string]string{
				"Content-Type":  "application/x-www-form-urlencoded",
				"User-Agent":    "GokismetTest/1.0 | Gokismet/2.0",
				"From":          "first.last@gmail.com",
				"Cache-Control": "no-cache",
			},
		},
	}

	for i, test := range data {

		client := NewVerifyingTestClient()

		var wclient gokismet.Client = client
		for _, headers := range test.Wrappers {
			wclient = gokismet.WrapClient(wclient, headers)
		}

		api := gokismet.NewAPIWithClient(TESTAPIKEY, TESTSITE, wclient)
		fn(api, nil)

		// We expect 2 client requests: one for key verification and
		// one for the actual API call.
		if len(client.Requests) != 2 {
			t.Fatalf("Test %d: Expected 2 client requests, got %d", i+1, len(client.Requests))
		}

		// Both requests should have custom HTTP headers. Test both.
		for j, req := range client.Requests {
			errors := compareKeyValuePairs(test.Expected, req.HeaderItems)
			for _, err := range errors {
				t.Errorf("Test %d, Request %d: %s", i+1, j+1, err)
			}
		}
	}
}

// TestAPIErrorString tests string formatting for the
// APIError type (mainly just for code coverage).
func TestAPIErrorString(t *testing.T) {

	data := []struct {
		Call     gokismet.APICall
		Result   string
		Help     string
		Expected string
	}{
		{
			Call:     gokismet.APIVerifyKey,
			Expected: `Akismet returned an empty string`,
		},
		{
			Call:     gokismet.APIVerifyKey,
			Help:     "A helpful diagnostic message",
			Expected: `Akismet returned an empty string (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APIVerifyKey,
			Result:   "invalid",
			Expected: `Akismet returned "invalid"`,
		},
		{
			Call:     gokismet.APIVerifyKey,
			Result:   "invalid",
			Help:     "A helpful diagnostic message",
			Expected: `Akismet returned "invalid" (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APICheckComment,
			Expected: `Check Comment returned an empty string`,
		},
		{
			Call:     gokismet.APICheckComment,
			Help:     "A helpful diagnostic message",
			Expected: `Check Comment returned an empty string (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APICheckComment,
			Result:   "invalid",
			Expected: `Check Comment returned "invalid"`,
		},
		{
			Call:     gokismet.APICheckComment,
			Result:   "invalid",
			Help:     "A helpful diagnostic message",
			Expected: `Check Comment returned "invalid" (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APISubmitHam,
			Expected: `Submit Ham returned an empty string`,
		},
		{
			Call:     gokismet.APISubmitHam,
			Help:     "A helpful diagnostic message",
			Expected: `Submit Ham returned an empty string (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APISubmitHam,
			Result:   "invalid",
			Expected: `Submit Ham returned "invalid"`,
		},
		{
			Call:     gokismet.APISubmitHam,
			Result:   "invalid",
			Help:     "A helpful diagnostic message",
			Expected: `Submit Ham returned "invalid" (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APISubmitSpam,
			Expected: `Submit Spam returned an empty string`,
		},
		{
			Call:     gokismet.APISubmitSpam,
			Help:     "A helpful diagnostic message",
			Expected: `Submit Spam returned an empty string (A helpful diagnostic message)`,
		},
		{
			Call:     gokismet.APISubmitSpam,
			Result:   "invalid",
			Expected: `Submit Spam returned "invalid"`,
		},
		{
			Call:     gokismet.APISubmitSpam,
			Result:   "invalid",
			Help:     "A helpful diagnostic message",
			Expected: `Submit Spam returned "invalid" (A helpful diagnostic message)`,
		},
	}

	for i, test := range data {

		err := &gokismet.APIError{
			Call:   test.Call,
			Result: test.Result,
			Help:   test.Help,
		}

		if got := err.Error(); got != test.Expected {
			t.Errorf("Test %d: Expected error %q, got %q", i+1, test.Expected, got)
		}
	}
}

// TestAuthErrorString tests string formatting for the
// AuthError type (mainly just for code coverage).
func TestAuthErrorString(t *testing.T) {

	data := []struct {
		Key    string
		Site   string
		Result string
		Help   string
		Error  string
	}{
		{
			Key:    TESTAPIKEY,
			Site:   TESTSITE,
			Result: "invalid",
			Error:  `Akismet failed to verify key "123456789abc" for site "http://www.example.com"`,
		},
		{
			Key:    TESTAPIKEY,
			Site:   TESTSITE,
			Result: "invalid",
			Help:   "A helpful diagnostic message",
			Error:  `Akismet failed to verify key "123456789abc" for site "http://www.example.com" (A helpful diagnostic message)`,
		},
	}

	for i, test := range data {

		err := &gokismet.AuthError{
			Key:    test.Key,
			Site:   test.Site,
			Result: test.Result,
			Help:   test.Help,
		}

		if msg := err.Error(); msg != test.Error {
			t.Errorf("Test %d: Expected AuthError %q, got %q", i+1, test.Error, msg)
		}
	}
}
