package gokismet

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// UA is the user agent used by gokismet to identify itself
// to the Akismet REST API. By default, all calls to Akismet
// include this value in the HTTP request header. To override
// it, use a custom Client (see WrapClient for an example).
const UA = "Gokismet/2.0"

// Akismet parameter keys.
const (
	pkKey           = "key"
	pkSite          = "blog"
	pkUserIP        = "user_ip"
	pkUserAgent     = "user_agent"
	pkReferer       = "referrer"
	pkPage          = "permalink"
	pkPageTimestamp = "comment_post_modified_gmt"
	pkType          = "comment_type"
	pkAuthor        = "comment_author"
	pkAuthorEmail   = "comment_author_email"
	pkAuthorPage    = "comment_author_url"
	pkContent       = "comment_content"
	pkTimestamp     = "comment_date_gmt"
	pkSiteLanguage  = "blog_lang"
	pkSiteCharset   = "blog_charset"
)

// Elements of an Akismet API Request.
// TODO: The API version is likely to change. Ideally we'd
// be able to update it without changing any code.
const (
	reqMethod       = "POST"
	reqScheme       = "https"
	reqHost         = "rest.akismet.com"
	reqAPIVersion   = "1.1"
	reqVerifyKey    = "verify-key"
	reqCheckComment = "comment-check"
	reqSubmitHam    = "submit-ham"
	reqSubmitSpam   = "submit-spam"
	reqContentType  = "application/x-www-form-urlencoded"
)

// Akismet request headers.
const (
	hdrUserAgent   = "User-Agent"
	hdrContentType = "Content-Type"
)

// Akismet response headers.
const (
	hdrHelp   = "X-Akismet-Debug-Help"
	hdrProTip = "X-Akismet-Pro-Tip"
)

// Valid Akismet return values. All other values are
// considered errors.
const (
	respVerified  = "valid"
	respHam       = "false"
	respSpam      = "true"
	respSubmitted = "Thanks for making the web a better place."
	respDiscard   = "discard"
)

// An APICall represents a method of the Akismet REST API.
type APICall uint32

const (
	APIVerifyKey APICall = iota
	APICheckComment
	APISubmitHam
	APISubmitSpam
)

// A SpamStatus is the result of a spam check.
type SpamStatus uint32

// Note the two statuses for spam: StatusProbableSpam and
// StatusDefiniteSpam. These correspond to the two types of spam
// defined by Akismet: normal spam and "pervasive" spam. Clients
// may choose to treat the two types differently. The Akismet
// Wordpress plugin, for example, puts normal spam into a queue
// for review, while pervasive spam is discarded immediately.
//
// See http://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/
// for more on pervasive spam.
const (
	StatusUnknown SpamStatus = iota // indicates an error
	StatusHam
	StatusProbableSpam
	StatusDefiniteSpam
)

// API provides spam checking and error reporting via the Akismet
// REST API.
type API struct {
	key      string
	site     string
	client   Client
	verified bool
}

// NewAPI returns an API that uses the standard library's default
// HTTP client. The API key and website provided here are sent to
// Akismet for verification on the first call to CheckComment,
// SubmitHam, or SubmitSpam.
func NewAPI(key string, site string) *API {
	return NewAPIWithClient(key, site, http.DefaultClient)
}

// NewAPIWithClient is the same as NewAPI but the caller provides
// its own Client. Custom clients can be used to intercept HTTP
// requests and responses (e.g. to add logging or other middleware).
func NewAPIWithClient(key string, site string, client Client) *API {
	return &API{
		key:    key,
		site:   site,
		client: client,
	}
}

// SubmitHam notifies Akismet of legitimate comments flagged as
// spam by a previous call to CheckComment. It takes comment data
// in the form of key-value pairs. Aim to provide as many of the
// original values as possible.
func (api *API) SubmitHam(values map[string]string) error {
	return api.submit(APISubmitHam, values)
}

// SubmitSpam notifies Akismet of spam that a previous call to
// CheckComment failed to detect. It takes comment data in the
// form of key-value pairs. Aim to provide as many of the original
// values as possible.
func (api *API) SubmitSpam(values map[string]string) error {
	return api.submit(APISubmitSpam, values)
}

// CheckComment takes comment data in the form of key-value pairs
// and checks it for spam.
func (api *API) CheckComment(values map[string]string) (SpamStatus, error) {

	if err := api.verify(); err != nil {
		return StatusUnknown, err
	}

	result, header, err := api.execute(APICheckComment, values)
	if err != nil {
		return StatusUnknown, err
	}

	switch {
	case result == respHam:
		return StatusHam, nil
	case result == respSpam && header.Get(hdrProTip) == respDiscard:
		return StatusDefiniteSpam, nil
	case result == respSpam:
		return StatusProbableSpam, nil
	default:
		return StatusUnknown, newAPIError(APICheckComment, result, header)
	}
}

// submit is the implementation of the SubmitHam and
// SubmitSpam methods.
func (api *API) submit(call APICall, values map[string]string) error {

	if err := api.verify(); err != nil {
		return err
	}

	result, header, err := api.execute(call, values)
	if err != nil {
		return err
	}

	if result != respSubmitted {
		return newAPIError(call, result, header)
	}

	return nil
}

// verify authorises an API key and website with Akismet.
// All of the public API methods call verify first.
func (api *API) verify() error {

	if api.verified {
		return nil
	}

	values := map[string]string{
		pkKey:  api.key,
		pkSite: api.site,
	}

	result, header, err := api.execute(APIVerifyKey, values)
	if err != nil {
		return err
	}

	if result != respVerified {
		return newAuthError(api.key, api.site, result, header)
	}

	api.verified = true
	return nil
}

// url returns the Akismet REST URL for the given API call.
func (api *API) url(call APICall) string {

	var command string
	var qualify bool

	switch call {
	case APIVerifyKey:
		command = reqVerifyKey
	case APICheckComment:
		command = reqCheckComment
		qualify = true
	case APISubmitHam:
		command = reqSubmitHam
		qualify = true
	case APISubmitSpam:
		command = reqSubmitSpam
		qualify = true
	default:
		// If we reach this point there's a bug in our code.
		// Might as well fail fast.
		panic("url: Unknown API Call")
	}

	host := reqHost
	if qualify {
		host = api.key + "." + host
	}

	u := url.URL{
		Scheme: reqScheme,
		Host:   host,
		Path:   reqAPIVersion + "/" + command,
	}

	return u.String()
}

// encode URL-encodes the given key value pairs and
// concatenates them into a query string suitable for
// Akismet requests. If not provided, the verified website
// is added to the list of parameters.
func (api *API) encode(values map[string]string) string {

	uvals := url.Values{
		// Default the website to the value used for
		// API key verification.
		pkSite: {api.site},
	}

	for k, v := range values {
		if v != "" {
			uvals.Set(k, v)
		}
	}

	return uvals.Encode()
}

// request creates an HTTP Request from the provided endpoint
// URL and map of query parameters.
func (api *API) request(curl string, values map[string]string) (*http.Request, error) {

	req, err := http.NewRequest(reqMethod, curl, strings.NewReader(api.encode(values)))
	if err != nil {
		return nil, err
	}

	// Default HTTP headers.
	req.Header.Set(hdrContentType, reqContentType)
	req.Header.Set(hdrUserAgent, UA)

	return req, nil
}

// execute calls the given Akismet method with the given
// parameters and returns the HTTP Response body and headers.
func (api *API) execute(call APICall, values map[string]string) (string, http.Header, error) {

	curl := api.url(call)

	req, err := api.request(curl, values)
	if err != nil {
		return "", nil, err
	}

	resp, err := api.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, newAPIError(call, "Status "+resp.Status, resp.Header)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	return string(body), resp.Header, nil
}

// An APIError represents an error or unexpected response
// from the Akismet REST API.
type APIError struct {
	// Type of API call.
	Call APICall
	// Value returned by Akismet.
	Result string
	// Additional error info from Akismet (may be empty).
	Help string
}

func newAPIError(call APICall, result string, header http.Header) *APIError {
	return &APIError{
		Call:   call,
		Result: result,
		Help:   header.Get(hdrHelp),
	}
}

func (e APIError) Error() string {

	var s string

	switch e.Call {
	case APICheckComment:
		s = "Check Comment"
	case APISubmitHam:
		s = "Submit Ham"
	case APISubmitSpam:
		s = "Submit Spam"
	default:
		s = "Akismet"
	}

	s += " returned "

	if e.Result == "" {
		s += "an empty string"
	} else {
		s += "\"" + e.Result + "\""
	}

	if e.Help != "" {
		s += " (" + e.Help + ")"
	}

	return s
}

// An AuthError indicates that Akismet was unable to verify
// the provided API key.
type AuthError struct {
	// API key being verified.
	Key string
	// Website associated with the API key.
	Site string
	// Value returned by Akismet.
	Result string
	// Additional error info from Akismet (may be empty).
	Help string
}

func newAuthError(key string, site string, result string, header http.Header) *AuthError {
	return &AuthError{
		Key:    key,
		Site:   site,
		Result: result,
		Help:   header.Get(hdrHelp),
	}
}

func (e AuthError) Error() string {

	s := "Akismet failed to verify key \"" + e.Key + "\" for site \"" + e.Site + "\""

	if e.Help != "" {
		s += " (" + e.Help + ")"
	}

	return s
}

// A Client executes an HTTP request and returns an HTTP
// response. Its interface is satisfied by http.Client.
// Provide your own implementation to intercept gokismet's
// HTTP requests and responses.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// clientWithHeaders decorates a Client with custom request
// headers.
type clientWithHeaders struct {
	client  Client
	headers map[string]string
}

// WrapClient takes a Client and a map of key-value pairs and
// returns a new Client that injects those values into all
// HTTP request headers.
func WrapClient(client Client, headers map[string]string) Client {
	return &clientWithHeaders{
		client:  client,
		headers: headers,
	}
}

// Do injects this object's key-value pairs into the incoming
// request header and delegates to the Do method of the wrapped
// Client.
func (c *clientWithHeaders) Do(req *http.Request) (*http.Response, error) {

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

// A Comment represents a blog comment, forum post, or other
// item of (potentially spammy) user-generated content. When
// creating instances of this type, aim to set as many fields
// as possible. The more information Akismet has to work with,
// the more accurate its spam detection.
type Comment struct {

	// Homepage URL of the website being commented on.
	// Defaults to the site used for key verification.
	Site string

	// IP address of the commenter.
	UserIP string

	// User agent string of the commenter's browser.
	UserAgent string

	// The HTTP_REFERER header.
	Referer string

	// URL of the page being commented on.
	Page string

	// Publish date/time of the page being commented on.
	PageTimestamp time.Time

	// Name of the commenter.
	Author string

	// Email address of the commenter.
	AuthorEmail string

	// Website of the commenter.
	AuthorPage string

	// Comment type, e.g. "comment", "forum-post".
	// See https://blog.akismet.com/2012/06/19/pro-tip-tell-us-your-comment_type/
	// for more examples.
	Type string

	// Content of the comment. May contain HTML.
	Content string

	// Publish date/time of the comment. Akismet uses
	// the current time if one is not specified.
	Timestamp time.Time

	// Comma-separated list of languages in use on the
	// website being commented on, e.g. "en, fr_ca".
	SiteLanguage string

	// Character encoding for the website being commented
	// on, e.g. "UTF-8".
	SiteCharset string
}

// Values returns comment data as a map of key-value pairs
// which can then be passed to the API methods CheckComment,
// SubmitHam and SubmitSpam.
func (c *Comment) Values() map[string]string {

	var values map[string]string

	add := func(key, val string) {
		if val != "" {
			if values == nil {
				values = make(map[string]string)
			}
			values[key] = val
		}
	}

	add(pkUserIP, c.UserIP)
	add(pkUserAgent, c.UserAgent)
	add(pkReferer, c.Referer)
	add(pkPage, c.Page)
	add(pkPageTimestamp, formatTime(c.PageTimestamp))
	add(pkType, c.Type)
	add(pkAuthor, c.Author)
	add(pkAuthorEmail, c.AuthorEmail)
	add(pkAuthorPage, c.AuthorPage)
	add(pkContent, c.Content)
	add(pkTimestamp, formatTime(c.Timestamp))
	add(pkSite, c.Site)
	add(pkSiteCharset, c.SiteCharset)
	add(pkSiteLanguage, c.SiteLanguage)

	return values
}

func formatTime(t time.Time) string {

	if t.IsZero() {
		return ""
	}
	// Akismet requires UTC time in ISO 8601 format
	// e.g. "2016-04-18T09:30:59Z".
	return t.UTC().Format(time.RFC3339)
}
