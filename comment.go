package gokismet

import (
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
)

// A Comment represents an item of user-generated comment to be checked for
// spam, such as a blog comment or forum post. The zero-value object is not
// guaranteed to work. Always use one of the constructors to create Comments.
type Comment struct {
	api    *API
	params *url.Values
}

// NewComment creates a Comment with the provided Akismet API key and
// website. The key and website are verified with Akismet and stored
// for use in subsequent calls to Check, ReportSpam and ReportNotSpam. If
// Akismet fails to verify your key, NewComment returns a nil pointer and
// a non-nil error.
//
// NewComment takes an optional third argument, the name of your application.
// If provided it will be sent to Akismet as part of the user agent in any
// API calls. The preferred format is application name/version, e.g.
//
//     comment, err := gokismet.NewComment("YOUR_API_KEY", "http://yourwebsite.com", "YourApplication/1.0")
//
// Omit the application name to use the default user agent of "Gokismet/1.0".
//
//     comment, err := gokismet.NewComment("YOUR_API_KEY", "http://yourwebsite.com")
func NewComment(key string, site string, appName ...string) (*Comment, error) {
	return new(NewAPI, key, site, appName...)
}

// NewTestComment creates a Comment in test mode, meaning that Akismet
// will not learn and adapt its behaviour based on the Comment's API calls.
// Use this version of the constructor for development and testing.
//
// As with NewComment, the provided API key and website are verified with
// Akismet and stored for subsequent calls to Check, ReportSpam and
// ReportNotSpam. A non-nil error is returned if verification fails.
//
// NewTestComment also supports the optional application name argument (see
// NewComment).
func NewTestComment(key string, site string, appName ...string) (*Comment, error) {
	return new(NewTestAPI, key, site, appName...)
}

// new does the heavy lifting for NewComment and NewTestComment.
// It initialises a new Comment and verifies the provided Akismet API key.
// It will also optionally set the user agent on the api member. If
// everything works you get a new Comment, otherwise you get nil and
// a non-nil error object.
func new(newapi func() *API, key string, site string, appName ...string) (*Comment, error) {

	// Create a new Comment
	comment := &Comment{
		api: newapi(),
		params: &url.Values{
			_Site: {site},
			_Type: {"comment"},
		},
	}

	// Optionally set the user agent
	switch len(appName) {
	case 0:
		// No name provided - do nothing
	case 1:
		// Name provided - use it to set the user agent
		comment.api.SetUserAgent(appName[0])
	default:
		// More than one name provided - fail
		return nil, errors.New("multiple app names passed to NewComment: expected 0 or 1")
	}

	// Verify the API key and website
	err := comment.api.VerifyKey(key, site)
	if err != nil {
		return nil, err
	}

	return comment, nil
}

// Check sends a Comment to Akismet for spam checking. If the call is
// successful, the returned status is one of StatusNotSpam,
// StatusProbableSpam or StatusDefiniteSpam and the returned error is nil.
// Otherwise, Check returns StatusUnknown and a non-nil error.
//
// The Akismet docs advise sending as much information about a comment as
// possible. The more data you provide, the more accurate the results. In
// particular, the commenter's IP address must be set (Check will fail
// without it) and the user agent is highly recommended.
func (c *Comment) Check() (SpamStatus, error) {
	return c.api.CheckComment(c.params)
}

// ReportSpam notifies Akismet that something it thought was legitimate is
// actually spam. This implies that a previous call to Check returned
// StatusNotSpam. When calling ReportSpam you should provide as much of the
// comment data from the original Check call as possible. You may not be
// able to resend everything, but any values you do send should be identical
// to the previous values.
func (c *Comment) ReportSpam() error {
	return c.api.SubmitSpam(c.params)
}

// ReportNotSpam notifies Akismet that something it thought was spam is
// actually legitimate. This implies that a previous call to Check returned
// StatusProbableSpam or StatusDefiniteSpam. When calling ReportNotSpam you
// should provide as much of the comment data from the original Check call as
// possible. You may not be able to resend everything, but any values you do
// send should be identical to the previous values.
func (c *Comment) ReportNotSpam() error {
	return c.api.SubmitHam(c.params)
}

// Reset reverts a Comment to its initial state (i.e. just after the call
// to NewComment or NewTestComment).
func (c *Comment) Reset() {
	c.params = &url.Values{
		_Site: {c.params.Get(_Site)},
		_Type: {c.params.Get(_Type)},
	}
}

// DebugTo provides a Writer for debug output. Once set, the writer will
// be used to log all HTTP requests sent to Akismet and all HTTP responses
// received. For development and testing only!
func (c *Comment) DebugTo(writer io.Writer) {
	c.api.SetDebugWriter(writer)
}

// SetType specifies the type of content being checked for spam. The default
// value is "comment". See http://blog.akismet.com/2012/06/19/pro-tip-tell-us-your-comment_type/
// for other options.
func (c *Comment) SetType(s string) {
	c.set(_Type, s)
}

// SetUserIP specifies the IP address of the commenter.
// This is required for calls to Check, ReportSpam and ReportNotSpam.
func (c *Comment) SetUserIP(s string) {
	c.set(_UserIP, s)
}

// SetUserAgent specifies the user agent of the commenter's browser.
// This is not technically required but still highly recommended for
// calls to Check, ReportSpam and ReportNotSpam.
func (c *Comment) SetUserAgent(s string) {
	c.set(_UserAgent, s)
}

// SetReferer specifies the commenter's referring URL.
func (c *Comment) SetReferer(s string) {
	c.set(_Referer, s)
}

// SetPage specifies the URL of the page where the comment was entered.
func (c *Comment) SetPage(s string) {
	c.set(_Page, s)
}

// SetPageTimestamp specifies the publish date of the page where the
// comment was entered.
func (c *Comment) SetPageTimestamp(t time.Time) {
	c.set(_PageTimestamp, formatTime(t))
}

// SetAuthor specifies the name submitted by the commenter.
func (c *Comment) SetAuthor(s string) {
	c.set(_Author, s)
}

// SetEmail specifies the email address submitted by the commenter.
func (c *Comment) SetEmail(s string) {
	c.set(_Email, s)
}

// SetURL specifies the URL submitted by the commenter.
func (c *Comment) SetURL(s string) {
	c.set(_URL, s)
}

// SetContent specifies the content of the comment.
func (c *Comment) SetContent(s string) {
	c.set(_Content, s)
}

// SetTimestamp specifies the creation time of the comment. If this
// is not provided, Akismet uses the time of the API call.
func (c *Comment) SetTimestamp(t time.Time) {
	c.set(_Timestamp, formatTime(t))
}

// SetSiteLanguage specifies the language(s) in use on the site
// where the comment was entered. Format is ISO 639-1, comma-separated
// (e.g. "en, fr_ca").
func (c *Comment) SetSiteLanguage(s string) {
	c.set(_SiteLanguage, s)
}

// SetCharset specifies the character encoding for the comment data
// (e.g. "UTF-8" or "ISO-8859-1").
func (c *Comment) SetCharset(s string) {
	c.set(_Charset, s)
}

// Generic set param function safeguards against blank values
func (c *Comment) set(key string, value string) {
	if s := strings.TrimSpace(value); s != "" {
		c.params.Set(key, s)
	}
}

// formatTime returns a string representation of a Time object,
// formatted for Akismet API calls.
func formatTime(t time.Time) string {
	// Akismet requires UTC time in ISO 8601 format
	// e.g. "2015-04-18T10:30Z"
	return t.UTC().Format(time.RFC3339)
}
