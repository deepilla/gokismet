package gokismet

import (
	"io"
	"net/url"
	"strings"
	"time"
)

// A Comment represents an item of user-generated comment to be checked for
// spam, such as a blog comment or forum post. The zero-value object is not
// guaranteed to work. Always use one of the constructors to create Comments.
type Comment struct {
	api    API
	params url.Values
}

// NewComment creates a Comment with the provided Akismet API key and
// website. The key and website are verified with Akismet and stored
// for use in subsequent calls to Check, ReportSpam and ReportNotSpam. If
// Akismet fails to verify your key, NewComment returns a nil pointer and
// a non-nil error.
func NewComment(key string, site string) (*Comment, error) {
	return new(key, site, false, "")
}

// NewCommentUA is identical to NewComment except it allows you to specify
// a user agent to send to Akismet in API calls. The user agent should
// be the name of your application, preferably in the format application
// name/version, e.g.
//
//		MyApplication/1.0
//
// Note: This is distinct from SetUserAgent which sets the commenter's
// user agent for a specific comment.
func NewCommentUA(key string, site string, userAgent string) (*Comment, error) {
	return new(key, site, false, userAgent)
}

// NewTestComment creates a Comment in test mode. This tells Akismet
// not to learn from or adapt to any API calls it receives, making
// tests somewhat repeatable. Test mode is recommended (but not
// required) for development.
//
// As with NewComment, the provided API key and website are verified with
// Akismet and stored for subsequent calls to Check, ReportSpam and
// ReportNotSpam. A non-nil error is returned if verification fails.
func NewTestComment(key string, site string) (*Comment, error) {
	return new(key, site, true, "")
}

// NewTestCommentUA is identical to NewTestComment except it allows you to
// specify a user agent to send to Akismet in API calls. The user agent
// should be the name of your application, preferably in the format
// application name/version, e.g.
//
//		MyApplication/1.0
//
// Note: This is distinct from SetUserAgent which sets the commenter's
// user agent for a specific comment.
func NewTestCommentUA(key string, site string, userAgent string) (*Comment, error) {
	return new(key, site, true, userAgent)
}

// new does the heavy lifting for the various versions of the Comment
// constructor. It initialises a new Comment, sets its user agent, and
// verifies the provided Akismet API key. If the key is verified, new
// returns the new Comment, otherwise it returns nil with a non-nil error
// object.
func new(key string, site string, testMode bool, userAgent string) (*Comment, error) {

	comment := &Comment{
		api: API{
			TestMode:  testMode,
			UserAgent: userAgent,
		},
		params: url.Values{
			_Site: {site},
			_Type: {"comment"},
		},
	}

	if err := comment.api.VerifyKey(key, site); err != nil {
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
	return c.api.CheckComment(&c.params)
}

// ReportSpam tells Akismet that something it thought was legitimate
// content is actually spam. This implies that a previous call to Check
// returned StatusNotSpam. When calling ReportSpam you should provide as
// much of the comment data from the original Check call as possible.
// You may not be able to resend everything, but any values you do send
// should be identical to the previous values.
func (c *Comment) ReportSpam() error {
	return c.api.SubmitSpam(&c.params)
}

// ReportNotSpam tells Akismet that something it thought was spam is
// actually legitimate content. This implies that a previous call to Check
// returned StatusProbableSpam or StatusDefiniteSpam. When calling ReportNotSpam
// you should provide as much of the comment data from the original Check call
// as possible. You may not be able to resend everything, but any values you
// do send should be identical to the previous values.
func (c *Comment) ReportNotSpam() error {
	return c.api.SubmitHam(&c.params)
}

// Reset reverts a Comment to its initial state (i.e. just after the call
// to NewComment, NewTestComment etc).
func (c *Comment) Reset() {
	c.params = url.Values{
		_Site: {c.params.Get(_Site)},
		_Type: {c.params.Get(_Type)},
	}
}

// DebugTo specifies a Writer for debug output. Any HTTP requests sent to
// Akismet and HTTP responses received from Akismet will be logged to this
// Writer. As the name suggests, you should only enable this feature during
// development.
func (c *Comment) DebugTo(writer io.Writer) {
	c.api.DebugWriter = writer
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
