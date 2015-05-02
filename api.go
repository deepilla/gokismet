package gokismet

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
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

// These are the possible spam statuses. There are two statuses for spam
// because Akismet splits spam into two types: normal and "pervasive".
// Pervasive spam is the really blatant stuff.
//
// See http://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/
// for more on this distinction and how pervasive spam is treated in
// WordPress.
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

// String returns a human-readable description of a SpamStatus, e.g.
// for error-handling or debugging.
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

// NewAPIError returns a new APIError populated with the given reason and
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
		err += " Help: " + e.Help + "."
	}
	return err
}

// An API is a thin wrapper around the methods of the Akismet REST API.
// While it can be used directly, Comment is more convenient and the
// better choice in most cases.
type API struct {

	// A user agent to pass to Akismet in the HTTP request header.
	// This should be the name of the client application, in the
	// format application name/version number, e.g. "MyApplication/1.0".
	// If left unset, a default value of "Gokismet/1.0" is used.
	UserAgent string

	// Whether or not to call Akismet in test mode. In test mode,
	// Akismet doesn't learn and adapt based on your API calls,
	// making them somewhat repeatable. Test mode is recommended
	// (but not required) for development and testing.
	TestMode bool

	// A Writer for logging HTTP requests and responses. If set to
	// a non-nil value, all requests to and responses from Akismet
	// are written to it. Output is provided as a convenience for
	// development and testing only. It should not be used in
	// Production.
	Output io.Writer

	// The Akismet API key, set in VerifyKey and required for calls
	// to CheckComment, SubmitSpam and SubmitHam. Deliberately not
	// exported so that clients are forced to verify their keys.
	key string
}

// VerifyKey validates an API key with Akismet. It takes the key to be
// validated and the homepage url of the website where the key will be
// used. If verification succeeds, VerifyKey returns nil and stores the
// key for subsequent API calls. Otherwise, a non-nil error is returned.
// Must be called before CheckComment, SubmitSpam or SubmitHam.
//
// See http://akismet.com/development/api/#verify-key for the Akismet docs.
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
// possible query parameters. The following parameters are required:
//
//     blog           // the website where the comment was entered
//     user_ip        // the ip address of the commenter
//
// The following parameter is highly recommended:
//
//     user_agent     // the user agent of the commenter's browser
//
// Best practice is to provide as much comment information as possible.
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
// documentation. This method has no required query parameters but the
// following are highly recommended:
//
//     blog           // the website where the comment was entered
//     user_ip        // the ip address of the commenter
//     user_agent     // the user agent of the commenter's browser
//
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
// documentation. This method has no required query parameters but the
// following are highly recommended:
//
//     blog           // the website where the comment was entered
//     user_ip        // the ip address of the commenter
//     user_agent     // the user agent of the commenter's browser
//
// Best practice is to supply as many of the parameters that were
// originally sent to CheckComment as possible.
func (api *API) SubmitHam(params *url.Values) error {
	return api.submit("submit-ham", params)
}

// submit does the heavy lifting for SubmitHam and SubmitSpam. The method
// determines whether the comment data described in the query parameters
// is submitted as spam or ham. The returned error is non-nil in the event
// of an error.
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

	host := akismetHost
	if qualified {
		host = api.key + "." + host
	}

	u := url.URL{
		Scheme: akismetScheme,
		Host:   host,
		Path:   akismetVersion + "/" + method,
	}

	return u.String()
}

// buildRequest constructs an HTTP Request from the given url and query
// parameters.
func (api *API) buildRequest(u string, params *url.Values) (*http.Request, error) {

	// Create a new Request
	req, err := http.NewRequest("POST", u, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	// Assemble the request user agent
	ua := strings.TrimSpace(api.UserAgent)
	if ua != "" {
		ua += " | "
	}
	ua += defaultUserAgent

	// Customise the request headers
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req, nil
}

// doRequest executes an HTTP Request and returns the Response.
func (api *API) doRequest(req *http.Request) (*http.Response, error) {

	// Use the default client from the http package to execute our
	// request. That's what helper functions like http.Get do.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// execute queries the Akismet API and returns the response body and header.
// It takes an Akismet endpoint URL and a set of query parameters.
func (api *API) execute(u string, params *url.Values) (string, http.Header, error) {

	// In test mode we need to add an extra parameter to the request
	// but we'll remove it on exit so there are no side effects
	if api.TestMode && params.Get("is_test") == "" {
		params.Set("is_test", "1")
		defer params.Del("is_test")
	}

	// Construct a Request...
	req, err := api.buildRequest(u, params)
	if err != nil {
		return "", nil, err
	}

	// ...output it to the debugger...
	err = api.log(req)
	if err != nil {
		return "", nil, err
	}

	// ...and execute it
	resp, err := api.doRequest(req)
	if err != nil {
		return "", nil, err
	}

	// Output the Response to the debugger...
	err = api.log(resp)
	if err != nil {
		return "", nil, err
	}

	// ... and then read the response body, scheduling it to
	// close on function exit
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	return string(body), resp.Header, nil
}

// log writes the provided Request or Response to the designated
// output Writer, if supplied. The implementation of this function
// should not have any side effects on the Request/Response.
func (api *API) log(r interface{}) error {
	if api.Output != nil {
		return writeAndRestore(api.Output, r)
	}
	return nil
}

// writeAndRestore writes a Request or a Response to the supplied
// Writer. Unlike Request.Write and Response.Write, it restores the
// request/response body to its previous state afterwards. It's an
// ugly hack but it allows us to output the HTTP info (for debugging
// purposes) without having to worry about the side effects. If
// people heed the docs it will only ever be called in development.
func writeAndRestore(w io.Writer, r interface{}) error {

	var body *io.ReadCloser
	var write func(io.Writer) error

	// Get the body and write function from the Request/Response
	switch r := r.(type) {
	case *http.Request:
		body = &r.Body
		write = r.Write
	case *http.Response:
		body = &r.Body
		write = r.Write
	default:
		// Any type other than Request or Response is a no-op
		return nil
	}

	// Get the type name and apply some basic formatting so that
	// e.g. "*http.Response" becomes "Response"
	s := strings.Split(reflect.TypeOf(r).String(), ".")
	typ := s[len(s)-1]

	// Use our one and only read to save the body into a buffer.
	// We'll use this buffer to restore the body after any
	// destructive read or write operations
	buf, err := ioutil.ReadAll(*body)
	if err != nil {
		return err
	}

	// Restore the body after the call to ReadAll
	restoreBody(body, buf)

	// Output a header line before the call to Write
	_, err = io.WriteString(w, "\n\n["+strings.ToUpper(typ)+"]\n")
	if err != nil {
		return err
	}

	// Now call Write, but first schedule a restore on function exit
	// (because Write will close the body again)
	defer restoreBody(body, buf)
	return write(w)
}

// restoreBody resets the body of a Request/Response to buf
// after it has been closed by a read or write operation.
func restoreBody(body *io.ReadCloser, buf []byte) {
	*body = ioutil.NopCloser(bytes.NewReader(buf))
}
