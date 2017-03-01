/*
Package gokismet is a Go library for the Akismet anti-spam service.

Use gokismet to:

1. Check comments, forum posts, and other user-generated content for spam.

2. Notify Akismet of false positives (legitimate content incorrectly flagged
as spam) and false negatives (spam content that it failed to detect).

For background on the Akismet API, see https://akismet.com/development/api/#detailed-docs.
*/
package gokismet

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Akismet query string parameters.
const (
	paramKey           = "key"
	paramSite          = "blog"
	paramUserIP        = "user_ip"
	paramUserAgent     = "user_agent"
	paramReferer       = "referrer"
	paramPage          = "permalink"
	paramPageTimestamp = "comment_post_modified_gmt"
	paramType          = "comment_type"
	paramAuthor        = "comment_author"
	paramAuthorEmail   = "comment_author_email"
	paramAuthorSite    = "comment_author_url"
	paramContent       = "comment_content"
	paramTimestamp     = "comment_date_gmt"
	paramSiteLanguage  = "blog_lang"
	paramSiteCharset   = "blog_charset"
)

// Akismet API calls.
const (
	methodVerify     = "verify-key"
	methodCheck      = "comment-check"
	methodReportHam  = "submit-ham"
	methodReportSpam = "submit-spam"
)

// Expected Akismet return values. Any other values
// trigger a gokismet error.
const (
	responseVerified = "valid"
	responseHam      = "false"
	responseSpam     = "true"
	responseReported = "Thanks for making the web a better place."
)

// Useful Akismet response headers.
const (
	headerDebugHelp = "X-Akismet-Debug-Help"
	headerProTip    = "X-Akismet-Pro-Tip"
)

// Akismet's "pervasive" spam indicator, returned in the
// X-Akismet-Pro-Tip header.
const proTipDiscard = "discard"

// UserAgent identifies gokismet to the Akismet API. By default,
// all API calls include this value in the HTTP request header.
// Use a custom Client to override this behaviour (see ClientFunc
// for an example).
const UserAgent = "Gokismet/3.0"

// A SpamStatus is the result of a spam check. It represents
// Akismet's opinion on the spaminess of your content.
type SpamStatus uint32

// Note that there are two statuses for spam. They correspond to
// the two types of spam in Akismet.
// See https://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/
// for details.
const (
	// StatusUnknown means that an error occurred during
	// the spam check.
	StatusUnknown SpamStatus = iota

	// StatusHam means that Akismet did not detect spam
	// in your content.
	StatusHam

	// StatusProbableSpam means that Akismet thinks your
	// content is spam. But consider reviewing it before
	// permanently deleting.
	StatusProbableSpam

	// StatusDefiniteSpam means that Akismet is certain
	// your content is spam. It can be deleted without
	// review.
	StatusDefiniteSpam
)

// A Client is responsible for executing HTTP requests.
// Its interface is satisfied by http.Client. Provide
// your own implementation to intercept gokismet's
// requests and responses.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// A ClientFunc converts a standalone function into a Client.
// If f is a function with a matching signature, ClientFunc(f)
// is a Client that calls f as its Do method.
type ClientFunc func(req *http.Request) (*http.Response, error)

// Do calls a ClientFunc's underlying function.
func (f ClientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Checker provides spam checking and error reporting via
// the Akismet API.
type Checker struct {
	key      string
	site     string
	client   Client
	verified bool
}

// NewChecker returns a Checker that uses the given API key
// and website as credentials for the Akismet service. These
// credentials are verified automatically on the first call
// to any of the Checker methods.
//
// Checkers created with NewChecker use the default HTTP
// client to make calls to the Akismet API. To provide your
// own Client, use the NewCheckerClient function.
func NewChecker(key string, site string) *Checker {
	return NewCheckerClient(key, site, nil)
}

// NewCheckerClient is like NewChecker except the returned
// Checker uses the provided Client to make calls to the
// Akismet API. If the provided Client is nil, the default
// HTTP client is used instead.
func NewCheckerClient(key string, site string, client Client) *Checker {

	if client == nil {
		client = http.DefaultClient
	}

	return &Checker{
		key:    key,
		site:   site,
		client: client,
	}
}

// Check takes content in the form of key-value pairs and
// checks it for spam. If an error occurs, Check returns
// StatusUnknown and a non-nil error.
//
// Key-value pairs can either be constructed manually (see
// the Akismet docs for a list of valid keys) or generated
// with the Comment type. It's important to provide as many
// values as possible. The more data Akismet has to work
// with, the faster and more accurate its spam detection.
func (ch *Checker) Check(values map[string]string) (SpamStatus, error) {

	if !ch.verified {
		if err := ch.verify(); err != nil {
			return StatusUnknown, err
		}
		ch.verified = true
	}

	url := buildURL(methodCheck, ch.key)

	body, header, err := ch.call(url, values)
	if err != nil {
		return StatusUnknown, err
	}

	switch string(body) {
	case responseHam:
		return StatusHam, nil
	case responseSpam:
		if header.Get(headerProTip) == proTipDiscard {
			return StatusDefiniteSpam, nil
		}
		return StatusProbableSpam, nil
	default:
		return StatusUnknown, newValError(methodCheck, string(body), header)
	}
}

// ReportHam notifies Akismet of legitimate content incorrectly
// flagged as spam by the Check method. Like Check, it takes
// content in the form of key-value pairs. For best results,
// provide as many of the original values as possible.
func (ch *Checker) ReportHam(values map[string]string) error {
	return ch.report(methodReportHam, values)
}

// ReportSpam notifies Akismet of spam that the Check method
// failed to detect. Like Check, it takes content in the form
// of key-value pairs. For best results, provide as many of the
// original values as possible.
func (ch *Checker) ReportSpam(values map[string]string) error {
	return ch.report(methodReportSpam, values)
}

// report handles the heavy lifting for the ReportHam and
// ReportSpam methods.
func (ch *Checker) report(method string, values map[string]string) error {

	if !ch.verified {
		if err := ch.verify(); err != nil {
			return err
		}
		ch.verified = true
	}

	url := buildURL(method, ch.key)

	body, header, err := ch.call(url, values)
	if err != nil {
		return err
	}

	if string(body) != responseReported {
		return newValError(method, string(body), header)
	}

	return nil
}

// verify authenticates a Checker's API key and website.
func (ch *Checker) verify() error {

	// The verify-key endpoint is not qualified with an
	// API key so we pass a blank key to buildUrl.
	url := buildURL(methodVerify, "")

	values := map[string]string{
		paramKey:  ch.key,
		paramSite: ch.site,
	}

	body, header, err := ch.call(url, values)
	if err != nil {
		return err
	}

	if string(body) != responseVerified {
		return newKeyError(ch.key, ch.site, string(body), header)
	}

	return nil
}

// call makes a request to an Akismet endpoint with the given
// parameters and returns the response body and headers.
func (ch *Checker) call(url string, params map[string]string) ([]byte, http.Header, error) {

	defaultParams := map[string]string{
		paramSite: ch.site,
	}

	req, err := newRequest(url, mergeStringMaps(defaultParams, params))
	if err != nil {
		return nil, nil, err
	}

	resp, err := ch.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New(resp.Status + " returned from " + url)
	}

	body, err := ioutil.ReadAll(resp.Body)

	return body, resp.Header, err
}

// buildURL returns the Akismet endpoint URL for the
// given API method. If a non-empty API key is provided,
// the hostname is qualified with the key.
func buildURL(method string, key string) string {

	s := "https://"
	if key != "" {
		s += key + "."
	}
	return s + "rest.akismet.com/1.1/" + method
}

// newRequest creates an HTTP Request from the given
// endpoint URL and query parameters.
func newRequest(url string, params map[string]string) (*http.Request, error) {

	req, err := http.NewRequest("POST", url, strings.NewReader(encodeParams(params)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	return req, nil
}

// encodeParams converts key-value pairs into a URL-encoded
// query string suitable for use in an HTTP request.
func encodeParams(params map[string]string) string {

	values := url.Values{}

	for k, v := range params {
		if v != "" {
			values.Set(k, v)
		}
	}

	return values.Encode()
}

// mergeStringMaps creates a string to string map containing
// entries from the series of maps provided. Later values
// override earlier ones.
func mergeStringMaps(maps ...map[string]string) map[string]string {

	merge := func(dst, src map[string]string) map[string]string {
		for k, v := range src {
			if dst == nil {
				dst = make(map[string]string)
			}
			dst[k] = v
		}
		return dst
	}

	var values map[string]string

	for _, m := range maps {
		values = merge(values, m)
	}

	return values
}

// A ValError is the error returned by the Checker methods
// if Akismet returns an unexpected response. Typically it
// indicates a problem with the data sent to Akismet, e.g.
// a required value not being set.
type ValError struct {
	// The Akismet method being called.
	Method string
	// The value returned from Akismet.
	Response string
	// Any additional error info from Akismet (may be empty).
	Hint string
}

func newValError(method string, response string, header http.Header) *ValError {
	return &ValError{
		Method:   method,
		Response: response,
		Hint:     header.Get(headerDebugHelp),
	}
}

func (e ValError) Error() string {

	s := e.Method + " returned "

	if strings.TrimSpace(e.Response) == "" {
		s += "an empty string"
	} else {
		s += "\"" + e.Response + "\""
	}

	hint := e.Hint
	if hint == "" {
		switch e.Method {
		case methodCheck:
			hint = "expected true or false"
		case methodReportHam, methodReportSpam:
			hint = "expected a thank you message"
		}
	}

	if hint != "" {
		s += " (" + hint + ")"
	}

	return s
}

// A KeyError is the error returned by the Checker methods
// if Akismet fails to verify an API key.
type KeyError struct {
	// The API key being verified.
	Key string
	// The website associated with the API key.
	Site string
	// Details of the response from Akismet.
	*ValError
}

func newKeyError(key string, site string, response string, header http.Header) *KeyError {
	return &KeyError{
		Key:      key,
		Site:     site,
		ValError: newValError(methodVerify, response, header),
	}
}

func (e KeyError) Error() string {
	return "key " + e.Key + " not verified: " + e.ValError.Error()
}

// A Comment represents a chunk of content to be checked
// for spam, such as a blog comment or forum post.
//
// The Comment type provides a convenient way to generate
// key-value pairs for the Checker methods. However its use
// is completely optional.
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
	AuthorSite string

	// Content type, e.g. "comment", "forum-post".
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

// Values returns a Comment's data as a map of key-value
// pairs, suitable for use with the Checker methods.
func (c *Comment) Values() map[string]string {

	insert := func(dst map[string]string, key, value string) map[string]string {
		if value == "" {
			return dst
		}

		if dst == nil {
			dst = make(map[string]string)
		}
		dst[key] = value
		return dst
	}

	insertTime := func(dst map[string]string, key string, value time.Time) map[string]string {
		if value.IsZero() {
			return dst
		}
		// Akismet requires UTC time in ISO 8601 format
		// e.g. "2016-04-18T09:30:59Z".
		return insert(dst, key, value.UTC().Format(time.RFC3339))
	}

	var m map[string]string

	m = insert(m, paramUserIP, c.UserIP)
	m = insert(m, paramUserAgent, c.UserAgent)
	m = insert(m, paramReferer, c.Referer)
	m = insert(m, paramPage, c.Page)
	m = insertTime(m, paramPageTimestamp, c.PageTimestamp)
	m = insert(m, paramType, c.Type)
	m = insert(m, paramAuthor, c.Author)
	m = insert(m, paramAuthorEmail, c.AuthorEmail)
	m = insert(m, paramAuthorSite, c.AuthorSite)
	m = insert(m, paramContent, c.Content)
	m = insertTime(m, paramTimestamp, c.Timestamp)
	m = insert(m, paramSite, c.Site)
	m = insert(m, paramSiteCharset, c.SiteCharset)
	m = insert(m, paramSiteLanguage, c.SiteLanguage)

	return m
}
