package gokismet

import (
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

const commentUserAgent = "Gokismet.Comment"

var comment *Comment

// Check that a Comment has been created successfully and that it has the
// correct website, comment type, test mode and user agent settings.
func assertCommentObjectIsValid(
	t *testing.T, method string, comment *Comment, err error, testMode bool, userAgent string) {

	if err != nil {
		// We can't continue without a Comment object
		t.Fatalf("%s fail: %s returned '%s'", getFunctionName(2), method, err)
		return
	}
	// Check that we have the expected number of parameters
	if n, e := len(comment.params), 2; n != e {
		t.Errorf("%s fail: %s created a Comment with %d params, expected %d",
			getFunctionName(2),
			method,
			n,
			e,
		)
	}
	// Check that the site is correctly set
	if v, e := comment.params.Get("blog"), config.Site; v != e {
		t.Errorf("%s fail: %s created a Comment with Site '%s', expected '%s'",
			getFunctionName(2),
			method,
			v,
			e,
		)
	}
	// Check that comment type is correctly set
	if v, e := comment.params.Get("comment_type"), "comment"; v != e {
		t.Errorf("%s fail: %s created a Comment with comment type '%s', expected '%s'",
			getFunctionName(2),
			method,
			v,
			e,
		)
	}
	// Check that TestMode is correctly set
	if v, e := comment.api.TestMode, testMode; v != e {
		t.Errorf("%s fail: %s created a Comment with TestMode '%s', expected '%s'",
			getFunctionName(2),
			method,
			strconv.FormatBool(v),
			strconv.FormatBool(e),
		)
	}
	// Check that UserAgent is correctly set
	if v, e := comment.api.UserAgent, userAgent; v != e {
		t.Errorf("%s fail: %s created a Comment with UserAgent '%s', expected '%s'",
			getFunctionName(2),
			method,
			v,
			e,
		)
	}
}

// Check that a Comment has the string value we expect for the given parameter type.
func assertStringParameterEquals(t *testing.T, comment *Comment, key string, expected string) {
	if v := comment.params.Get(key); v != expected {
		t.Errorf("%s fail: Parameter '%s' is '%s', expected '%s'",
			getFunctionName(2),
			key,
			v,
			expected,
		)
	}
}

// Check that a Comment has the date value we expect for the given parameter type.
func assertDateParameterEquals(t *testing.T, comment *Comment, key string, expected time.Time) {
	assertStringParameterEquals(t, comment, key, formatTime(expected))
}

// Check that a Comment is sending output to the Writer we expect
func assertCommentLogsTo(t *testing.T, comment *Comment, expected io.Writer) {
	if w := comment.api.Output; w != expected {
		t.Errorf("%s fail: Comment is logging to %T %v, epected %T %v",
			getFunctionName(2),
			w,
			w,
			expected,
			expected,
		)
	}
}

// Confirm that the constructors create Comments with the expected settings.
// Note: The last version will be the Comment that gets used in subsequent tests.
func TestCommentCreate(t *testing.T) {
	var err error

	// Make sure creation fails with a dodgy API key
	comment, err = NewTestComment(badKey, config.Site)
	assertBadKeyIsNotVerified(t, "Comment.NewTestComment", badKey, err, comment)

	// Test all of the Comment constructors
	comment, err = NewComment(config.APIKey, config.Site)
	assertCommentObjectIsValid(t, "Comment.NewComment", comment, err, false, "")

	comment, err = NewTestComment(config.APIKey, config.Site)
	assertCommentObjectIsValid(t, "Comment.NewTestComment", comment, err, true, "")

	comment, err = NewCommentUA(config.APIKey, config.Site, commentUserAgent)
	assertCommentObjectIsValid(t, "Comment.NewCommentUA", comment, err, false, commentUserAgent)

	comment, err = NewTestCommentUA(config.APIKey, config.Site, commentUserAgent)
	assertCommentObjectIsValid(t, "Comment.NewTestCommentUA", comment, err, true, commentUserAgent)

	// Add debugging
	var output io.Writer // zero-value is nil
	if config.Debug {
		output = os.Stdout
	}
	assertCommentLogsTo(t, comment, nil)
	comment.LogTo(output)
	assertCommentLogsTo(t, comment, output)
}

// Confirm that the Set... functions work as expected.
// Note: The values used in this function should be non-spammy as we check
// this object for spam in the next test and we expect a negative result.
func TestCommentParameters(t *testing.T) {
	var s string
	var ts time.Time

	if comment == nil {
		// We can't continue without a Comment object
		t.Fatalf("%s fail: The global Comment object has not been initialised.", getFunctionName(1))
	}

	s = config.IP
	comment.SetUserIP(s)
	assertStringParameterEquals(t, comment, "user_ip", s)

	s = config.UserAgent
	comment.SetUserAgent(s)
	assertStringParameterEquals(t, comment, "user_agent", s)

	s = "http://www.google.com"
	comment.SetReferer(s)
	assertStringParameterEquals(t, comment, "referrer", s)

	s = config.Article
	comment.SetPage(s)
	assertStringParameterEquals(t, comment, "permalink", s)

	// Set article timestamp to 1 month ago
	ts = time.Now().AddDate(0, -1, 0)
	comment.SetPageTimestamp(ts)
	assertDateParameterEquals(t, comment, "comment_post_modified_gmt", ts)

	s = "gokismet test"
	comment.SetAuthor(s)
	assertStringParameterEquals(t, comment, "comment_author", s)

	// Specifiying the email address has side effects. It seems to make
	// the following submit-ham call work, so when this test is run more
	// than once, the spammy comment is not detected as spam any more.
	// We can get around this by using a different email for the submit
	// tests.
	s = "check@example.com"
	comment.SetEmail(s)
	assertStringParameterEquals(t, comment, "comment_author_email", s)

	s = "This is an example comment that does not contain anything spammy. In the absence of other dodgy settings Akismet should return a negative (non-spam) response..."
	comment.SetContent(s)
	assertStringParameterEquals(t, comment, "comment_content", s)

	s = "http://www.example.com"
	comment.SetURL(s)
	assertStringParameterEquals(t, comment, "comment_author_url", s)

	// Set comment timestamp to current time
	ts = time.Now()
	comment.SetTimestamp(ts)
	assertDateParameterEquals(t, comment, "comment_date_gmt", ts)

	s = "en_us"
	comment.SetSiteLanguage(s)
	assertStringParameterEquals(t, comment, "blog_lang", s)

	s = "UTF-8"
	comment.SetCharset(s)
	assertStringParameterEquals(t, comment, "blog_charset", s)

	// Switch comment type to test SetType
	s = "forum-post"
	comment.SetType(s)
	assertStringParameterEquals(t, comment, "comment_type", s)

	// But remember to switch it back again before continuing
	s = "comment"
	comment.SetType(s)
	assertStringParameterEquals(t, comment, "comment_type", s)
}

// Confirm that spam checks return the expected results.
func TestCommentCheck(t *testing.T) {
	if comment == nil {
		// We can't continue without a Comment object
		t.Fatalf("%s fail: The global Comment object has not been initialised.", getFunctionName(1))
	}

	// Test the non-spammy comment set up in the previous function
	status, err := comment.Check()
	assertSpamStatusEquals(t, "Comment.Check", StatusNotSpam, status, err)

	// Make the comment spammy
	comment.SetAuthor("viagra-test-123")

	// And test again
	status, err = comment.Check()
	assertSpamStatusEquals(t, "Comment.Check", StatusProbableSpam, status, err)

	// Make the comment non-spammy again
	comment.SetAuthor("gokismet test")

	// And test a final time
	status, err = comment.Check()
	assertSpamStatusEquals(t, "Comment.Check", StatusNotSpam, status, err)
}

func TestCommentReport(t *testing.T) {
	if comment == nil {
		// We can't continue without a Comment object
		t.Fatalf("%s fail: The global Comment object has not been initialised.", getFunctionName(1))
	}

	// Change the email address, otherwise these calls might
	// affect subsequent calls to Comment.Check.
	comment.SetEmail("submit@example.com")

	if err := comment.ReportSpam(); err != nil {
		t.Errorf("%s fail: Comment.ReportSpam returned '%s'", getFunctionName(1), err)
	}

	if err := comment.ReportNotSpam(); err != nil {
		t.Errorf("%s fail: Comment.ReportNotSpam returned '%s'", getFunctionName(1), err)
	}
}

func TestCommentReset(t *testing.T) {
	if comment == nil {
		// We can't continue without a Comment object
		t.Fatalf("%s fail: The global Comment object has not been initialised.", getFunctionName(1))
	}

	// Reset should give us a Comment that looks like it was just created by
	// a constructor (in this case NewTestCommentUA)
	comment.Reset()
	assertCommentObjectIsValid(t, "Comment.Reset", comment, nil, true, commentUserAgent)

	// Reload the Comment
	TestCommentParameters(t)

	// Change the comment type and reset again
	// This time when we rest the comment type should be equal to
	// whatever we set it to be here
	s := "pingback"
	comment.SetType(s)
	assertStringParameterEquals(t, comment, "comment_type", s)
	comment.Reset()
	assertStringParameterEquals(t, comment, "comment_type", s)

	// Resetting the comment type to the default should give us a fresh
	// Comment again
	comment.SetType("comment")
	assertCommentObjectIsValid(t, "Comment.Reset", comment, nil, true, commentUserAgent)
}
