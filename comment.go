package gokismet

import (
	"io"
	"net/url"
	"strings"
	"time"
)

// A Comment represents a blog comment to be checked for spam. The zero-value
// object is not guaranteed to work. Always use one of the constructors to
// create your Comment objects.
type Comment struct {
	api    *API
	params *url.Values
}

// NewComment creates a Comment object with the provided Akismet API key
// and website. The key and website are verified with Akismet and stored
// for use in subsequent calls to Check, ReportSpam and ReportNotSpam. A
// non-nil error is returned if verification fails.
func NewComment(key string, site string) (*Comment, error) {
	return new(NewAPI, key, site)
}

// NewTestComment creates a Comment object in test mode, meaning that Akismet
// will not learn and adapt its behaviour based on the object's API calls.
// Use this version of the constructor for development and testing.
//
// As with NewComment, the provided API key and website are verified with
// Akismet and stored for subsequent calls to Check, ReportSpam and
// ReportNotSpam. A non-nil error is returned if verification fails.
func NewTestComment(key string, site string) (*Comment, error) {
	return new(NewTestAPI, key, site)
}

// new does the heavy lifting for NewComment and NewTestComment.
// It initialises a new Comment object and verifies the Akismet API key.
// If everything works you get a shiny new Comment object, otherwise you
// get nil and a non-nil error object.
func new(newapi func() *API, key string, site string) (*Comment, error) {
	comment := &Comment{
		api: newapi(),
		params: &url.Values{
			_Site: {site},
			_Type: {"comment"},
		},
	}
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
// particular, the commenter's IP address must be included (Check will fail
// without it) and the user agent is highly recommended.
func (c *Comment) Check() (SpamStatus, error) {
	return c.api.CheckComment(c.params)
}

// ReportSpam notifies Akismet that a comment it thought was legitimate is
// actually spam. This implies that a previous call to Check returned
// StatusNotSpam. When calling ReportSpam you should provide as much of the
// comment data from the original Check call as possible. You may not be
// able to resend everything, but any values you do send should be identical
// to the previous values.
func (c *Comment) ReportSpam() error {
	return c.api.SubmitSpam(c.params)
}

// ReportNotSpam notifies Akismet that a comment it thought was spam is actually
// a legitimate comment. This implies that a previous call to Check returned
// StatusProbableSpam or StatusDefiniteSpam. When calling ReportNotSpam you
// should provide as much of the comment data from the original Check call as
// possible. You may not be able to resend everything, but any values you do
// send should be identical to the previous values.
func (c *Comment) ReportNotSpam() error {
	return c.api.SubmitHam(c.params)
}

// Reset reverts a Comment object to its initial state (i.e. just after
// the call to NewComment or NewTestComment).
func (c *Comment) Reset() {
	c.params = &url.Values{
		_Site: {c.params.Get(_Site)},
		_Type: {c.params.Get(_Type)},
	}
}

// SetApplication updates the user agent sent to Akismet in API calls.
// The preferred format is application name/version, e.g.
//
//    MyApplication/1.0
func (c *Comment) SetApplication(s string) {
	c.api.SetApplication(s)
}

// SetDebugWriter specifies a Writer object for debug output. When set,
// the writer will be used to log the HTTP requests and responses sent
// to and received from Akismet.
func (c *Comment) SetDebugWriter(writer io.Writer) {
	c.api.writer = writer
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

// SetPermalink specifies the URL of the page where the comment was entered.
func (c *Comment) SetPermalink(s string) {
	c.set(_Permalink, s)
}

// SetArticleTime specifies the publish date of the page where the comment
// was entered.
func (c *Comment) SetArticleTime(t time.Time) {
	c.set(_ArticleTime, formatTime(t))
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

// SetCreateTime specifies the creation time of the comment. If this is not
// set, Akismet defaults to the time of the API call.
func (c *Comment) SetCreateTime(t time.Time) {
	c.set(_CreateTime, formatTime(t))
}

// SetSiteLanguage specifies the language(s) in use on the site where the
// commented was entered. Format is ISO 639-1, comma-separated, e.g. "en, fr_ca".
func (c *Comment) SetSiteLanguage(s string) {
	c.set(_SiteLanguage, s)
}

// SetCharset specifies the character encoding for the comment data,
// e.g. "UTF-8" or "ISO-8859-1".
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
