package gokismet

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	version          = "1.0"
	defaultUserAgent = "Gokismet/" + version

	akismetScheme  = "https"
	akismetHost    = "rest.akismet.com"
	akismetVersion = "1.1"
)

// Akismet parameter names
const (
	_Key       = "key"
	_Site      = "blog"
	_UserIP    = "user_ip"
	_UserAgent = "user_agent"
	// Akismet uses the correct spelling of the word "referrer".
	// We're using the HTTP misspelling for consistency with Golang.
	_Referer       = "referrer"
	_Page          = "permalink"
	_PageTimestamp = "comment_post_modified_gmt"
	_Type          = "comment_type"
	_Author        = "comment_author"
	_Email         = "comment_author_email"
	_URL           = "comment_author_url"
	_Content       = "comment_content"
	_Timestamp     = "comment_date_gmt"
	_SiteLanguage  = "blog_lang"
	_Charset       = "blog_charset"
)

// SpamStatus represents the result of a spam check.
type SpamStatus uint

// These are the possible spam statuses. Akismet distinguishes between two
// types of spam: normal spam and "pervasive" spam. Pervasive spam is the
// worst, most blatant type of spam (see http://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/
// for more info). Gokismet represents that distinction with the statuses
// ProbableSpam and DefiniteSpam.
const (
	// StatusUnknown is a default status indicating an error
	StatusUnknown SpamStatus = iota
	// StatusNotSpam means that Akismet did not detect any spam
	StatusNotSpam
	// StatusProbableSpam means that Akismet detected normal spam
	StatusProbableSpam
	// StatusDefiniteSpam means that Akismet detected "pervasive" spam
	StatusDefiniteSpam
)

// String returns a human-readable description of a SpamStatus
// (useful for error-handling or debugging).
func (s SpamStatus) String() string {
	switch s {
	case StatusUnknown:
		return "Unknown"
	case StatusNotSpam:
		return "Not Spam"
	case StatusProbableSpam:
		return "Probable Spam"
	case StatusDefiniteSpam:
		return "Definite Spam"
	}
	return "Invalid Status"
}

var errKeyNotVerified = errors.New("API key has not been verified")

// An APIError represents an error or unexpected result encountered
// during a call to the Akismet API.
type APIError struct {
	// Reason is a freeform description of the error
	Reason string
	// Result is the value returned by Akismet in the response body
	Result string
	// Help is the value of the X-akismet-debug-help response header
	Help string
	// Err is the value of the X-akismet-error response header
	Err string
	// AlertCode is the value of the X-akismet-alert-code response header
	AlertCode string
	// AlertMessage is the value of the X-akismet-alert-msg response header
	AlertMessage string
}

// NewAPIError returns a new APIError populated with the provided reason and
// result, plus any additional information from the provided response header.
func NewAPIError(reason string, result string, header *http.Header) APIError {
	return APIError{
		Reason: reason,
		Result: result,
		// Store any additional info from the Akismet response headers
		Help: header.Get("X-Akismet-Debug-Help"),
		// The following header values are undocumented. It's not clear
		// why or when they're returned or what values they contain. So
		// for now we're storing them without using them.
		Err:          header.Get("X-Akismet-Error"),
		AlertCode:    header.Get("X-Akismet-Alert-Code"),
		AlertMessage: header.Get("X-Akismet-Alert-Msg"),
	}
}

// Error implements the error interface.
func (e APIError) Error() string {
	err := e.Reason + "."
	if e.Result != "" {
		err += " Result: " + e.Result + "."
	}
	if e.Help != "" {
		err += " Info: " + e.Help + "."
	}
	return err
}

// An API is a thin wrapper around the methods of the Akismet REST API.
// While it can be used directly, the Comment class is more convenient
// in most cases.
//
// The zero value API object is not guaranteed to work. Clients should
// use one of the constructors to create new APIs.
type API struct {
	// The Akismet API key
	key string
	// The user agent passed to Akismet in API calls
	userAgent string
	// Whether or not to call Akismet in test mode. In test mode,
	// Akismet won't adapt its behaviour to your API calls.
	testMode bool
	// A writer for debugging. When provided, any Akismet requests
	// and responses will be logged to it.
	writer io.Writer
}

// NewAPI returns a new API.
func NewAPI() *API {
	return &API{
		userAgent: defaultUserAgent,
	}
}

// NewTestAPI returns a new API in test mode. API calls made in test mode
// do not trigger Akismet's "learning" behaviour, making tests somewhat
// repeatable. Test mode is recommended (but not required) for development
// and testing.
func NewTestAPI() *API {
	api := NewAPI()
	api.testMode = true
	return api
}

// SetUserAgent updates the user agent sent to Akismet in API calls.
// The preferred format is application name/version, e.g.
//
//    MyApplication/1.0
func (api *API) SetUserAgent(name string) {
	if s := strings.TrimSpace(name); s != "" {
		api.userAgent = s + " | " + defaultUserAgent
	}
}

// SetDebugWriter specifies a Writer for debug output. When set, the
// writer will be used to log any HTTP requests and responses sent to
// and received from Akismet. For development and testing only!
func (api *API) SetDebugWriter(writer io.Writer) {
	api.writer = writer
}

// VerifyKey validates an API key with Akismet. It takes the key to be
// validated and the homepage url of the website where the key will be
// used. If verification succeeds, VerifyKey returns nil and stores the
// key for subsequent API calls. Otherwise, a non-nil error is returned.
//
// Note: VerifyKey must be called before CheckComment, SubmitSpam
// or SubmitHam.
//
// See http://akismet.com/development/api/#verify-key for more info.
func (api *API) VerifyKey(key string, site string) error {

	u := api.buildRequestURL("verify-key", false)

	params := url.Values{
		_Key:  {key},
		_Site: {site},
	}

	result, header, err := api.execute(u, &params)
	if err != nil {
		return err
	}

	// Akismet returns "valid" if it successfully verifies a key
	if result == "valid" {
		// The API key's good - store it for subsequent calls
		api.key = key
		return nil
	}

	// Any other return value indicates a failure
	return NewAPIError("Akismet didn't verify key "+key, result, &header)
}

// CheckComment checks a comment for spam. It takes a set of query parameters
// describing the comment data and returns a SpamStatus and an error. If the
// call succeeds, the returned status is one of StatusNotSpam,
// StatusProbableSpam, or StatusDefiniteSpam and the returned error is nil.
// Otherwise, CheckComment returns StatusUnknown and a non-nil error.
// You must make a successful call to VerifyKey before calling CheckComment.
//
// See http://akismet.com/development/api/#comment-check for a list of
// possible query parameters. According to the docs the following parameters
// are required:
//
//     blog           // the website where the comment was entered
//     user_ip        // the ip address of the commenter
//     user_agent     // the user agent of the commenter's browser
//
// In practice only the website and IP address are genuinely required.
// CheckComment will fail if either is missing. User agent is not
// mandatory but it's highly advised. Akismet's results may be less
// accurate if it's not provided. Best practice is to provide as much
// data about the comment as possible.
func (api *API) CheckComment(params *url.Values) (SpamStatus, error) {

	if api.key == "" {
		return StatusUnknown, errKeyNotVerified
	}

	u := api.buildRequestURL("comment-check", true)

	result, header, err := api.execute(u, params)
	if err != nil {
		return StatusUnknown, err
	}

	// Akismet returns "true" if it detects spam
	if result == "true" {
		// The most blatant spam is indicated by a custom
		// response header
		if header.Get("X-Akismet-Pro-Tip") == "discard" {
			return StatusDefiniteSpam, nil
		}
		return StatusProbableSpam, nil
	}

	// Akismet returns "false" if it doesn't detect spam
	if result == "false" {
		return StatusNotSpam, nil
	}

	// Any other return value indicates a failure
	return StatusUnknown,
		NewAPIError("Akismet comment check failed", result, &header)
}

// SubmitSpam notifies Akismet of spam that it failed to detect in a
// previous call to CheckComment. It takes a set of query parameters
// (just like CheckComment) and returns a non-nil error if it fails.
//
// See http://akismet.com/development/api/#submit-spam for the Akismet
// documentation. Contrary to the docs, there are no required parameters
// for SubmitSpam but blog, user_id and user_agent are highly recommended.
// Best practice is to supply as many of the parameters that were
// originally sent to CheckComment as possible.
func (api *API) SubmitSpam(params *url.Values) error {
	return api.submit("submit-spam", params)
}

// SubmitHam notifies Akismet of a legitimate comment incorrectly flagged
// as spam in a previous call to CheckComment. It takes a set of query
// parameters (just like CheckComment) and returns a non-nil error if
// it fails.
//
// See http://akismet.com/development/api/#submit-ham for the Akismet
// documentation. Contrary to the docs, there are no required parameters
// for SubmitHam but blog, user_id and user_agent are highly recommended.
// Best practice is to supply as many of the parameters that were
// originally sent to CheckComment as possible.
func (api *API) SubmitHam(params *url.Values) error {
	return api.submit("submit-ham", params)
}

// submit does the heavy lifting for SubmitHam and SubmitSpam. The provided
// method name determines whether the comment data described in the query
// parameters is submitted as spam or ham. The returned error is non-nil
// in the event of an error.
func (api *API) submit(method string, params *url.Values) error {

	if api.key == "" {
		return errKeyNotVerified
	}

	u := api.buildRequestURL(method, true)

	result, header, err := api.execute(u, params)
	if err != nil {
		return err
	}

	// Akismet says thank you if the call succeeds
	if result == "Thanks for making the web a better place." {
		return nil
	}

	// Any other return value indicates failure
	return NewAPIError("Akismet "+strings.Replace(method, "-", " ", -1)+" failed", result, &header)
}

// buildRequestURL constructs the URL for an Akismet API call given an
// API method name and a flag indicating whether or not to qualify the
// domain with the API key.
func (api *API) buildRequestURL(method string, qualified bool) string {

	var host string

	if qualified == true {
		host = api.key + "." + akismetHost
	} else {
		host = akismetHost
	}

	u := url.URL{
		Scheme: akismetScheme,
		Host:   host,
		Path:   akismetVersion + "/" + method,
	}

	return u.String()
}

// buildRequest constructs an HTTP Request from a url and a set of request
// parameters.
func (api *API) buildRequest(u string, params *url.Values) (*http.Request, error) {

	// Get a new Request
	req, err := http.NewRequest("POST", u, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	// Customise the request headers
	req.Header.Set("User-Agent", api.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req, nil
}

// doRequest executes an HTTP Request and returns the Response.
func (api *API) doRequest(req *http.Request) (*http.Response, error) {

	// Use the http package's default client
	// to execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// execute queries the Akismet API and returns the response body and header.
// It takes an Akismet endpoint URL and a set of query parameters.
//
// execute uses named return arguments - they make returning 3 values
// a bit tidier.
func (api *API) execute(u string, params *url.Values) (result string, header http.Header, err error) {

	// Are we in test mode?
	// If so, we need to add an extra parameter to the request
	if api.testMode == true {
		params.Set("is_test", "1")
	}

	// Construct a Request...
	req, err := api.buildRequest(u, params)
	if err != nil {
		return
	}

	// ...output it to the debugger...
	if api.writer != nil {
		err = writeRequest(api.writer, req)
		if err != nil {
			return
		}
	}

	// ...and execute it
	resp, err := api.doRequest(req)
	if err != nil {
		return
	}

	// Output the Response to the debugger...
	if api.writer != nil {
		err = writeResponse(api.writer, resp)
		if err != nil {
			return
		}
	}

	// ... and then read the response body
	// (also set the body to close on function exit)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// Finally, set the return values and bounce
	result = string(body)
	header = resp.Header
	return
}

// writeRequest writes an HTTP Request to the supplied Writer. Unlike
// Request.Write it restores Request.Body to its former state afterwards.
func writeRequest(writer io.Writer, req *http.Request) error {
	_, err := io.WriteString(writer, "\n\n[REQUEST]\n")
	if err != nil {
		return err
	}
	return writeAndRestore(writer, req.Write, &req.Body)
}

// writeResponse writes an HTTP Response to the supplied Writer. Unlike
// Response.Write it restores Response.Body to its former state afterwards.
func writeResponse(writer io.Writer, resp *http.Response) error {
	_, err := io.WriteString(writer, "\n\n[RESPONSE]\n")
	if err != nil {
		return err
	}
	return writeAndRestore(writer, resp.Write, &resp.Body)
}

// writeAndRestore does the heavy lifting for writeRequest and writeResponse.
// It's a hack that allows Request and Response objects to be written out
// without killing the [Request|Response].Body. It's ugly but it should only
// be used during development/debugging only.
//
// TODO: Come up with a more elegant way to do this!
func writeAndRestore(writer io.Writer, write func(io.Writer) error, rc *io.ReadCloser) error {

	// Start by reading the ReadCloser into a buffer. We'll use
	// the buffer to restore the ReadCloser after any destructive
	// read or write operations...
	buf, err := ioutil.ReadAll(*rc)
	if err != nil {
		return err
	}

	// ...like the one we just did. The ReadCloser is now closed
	// thanks to ReadAll. We need to restore it before attempting
	// a write.
	setRC(rc, buf)

	// Now we can call the write function. But first we schedule
	// a restore on function exit because we know that the write
	// function will close the ReadCloser again.
	defer setRC(rc, buf)
	return write(writer)
}

// setRC initialises a ReadCloser using the supplied buffer.
// It can be called repeatedly without issues.
func setRC(rc *io.ReadCloser, buf []byte) {
	*rc = ioutil.NopCloser(bytes.NewReader(buf))
}
