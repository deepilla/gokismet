package gokismet

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	errMissingConfig = 100 + iota
	errInvalidConfig
	errIncompleteConfig
	errInvalidSite
)

type settings struct {
	// Akismet API Key
	APIKey string
	// Website to test for spam
	Site string
	// URL of an article on the provided website
	// We'll generate one if it isn't supplied
	Article string
	// IP Address to send in Akismet requests
	IP string
	// User Agent to send in Akismet requests
	UserAgent string
	// Whether or not to output Requests/Responses
	// to stdout
	Debug bool
}

var api *API
var comment *Comment

var config settings

// Test entry point
func TestMain(m *testing.M) {
	// Open the settings file
	f, err := os.Open("testconfig.json")
	if err != nil {
		os.Exit(errMissingConfig)
	}
	// Use the contents to populate the settings struct
	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		os.Exit(errInvalidConfig)
	}
	// Check that the settings have been populated correctly
	if config.APIKey == "" || config.Site == "" || config.IP == "" || config.UserAgent == "" {
		os.Exit(errIncompleteConfig)
	}
	// If there's no article URL specified in the config, make one up
	if config.Article == "" {
		u, err := url.Parse(config.Site)
		if err != nil {
			os.Exit(errInvalidSite)
		}
		u.Path = "blog/"
		u.RawQuery = "p=123"

		config.Article = u.String()
	}

	// Set up a debug writer. Even if we're not debugging, set
	// a dummy Writer object. That way we still test the logging
	// code path (we're just not outputting anything).
	debugger := ioutil.Discard
	if config.Debug {
		debugger = os.Stdout
	}

	// Create the API object
	api = &API{
		TestMode:    true,
		DebugWriter: debugger,
	}

	// Run the tests
	os.Exit(m.Run())
}

func defaultParams() url.Values {
	return url.Values{
		_Site:      {config.Site},
		_UserIP:    {config.IP},
		_UserAgent: {config.UserAgent},
		_Referer:   {"http://www.google.com"},
		_Page:      {config.Article},
		_Author:    {"gokismet tester"},
		_Type:      {"comment"},
		_Email:     {"hello@example.com"},
		_URL:       {"http://www.example.com"},
		_Content:   {"This is an example comment that does not contain anything spammy. In the absence of other settings Akismet should return a negative (non-spam) response for this comment. Cheers..."},
	}
}

func checkTestModeCleanup(t *testing.T, testName string, params *url.Values) {
	// Our global API object has TestMode = true
	// This means that an "is_test" parameter is added to the query
	// when calling the Akismet API. But it should be removed after
	// the call so we shouldn't see any sign of it now.
	if params.Get("is_test") != "" {
		t.Errorf("%s fail: Test Mode flag has not been removed", testName)
	}
}

func checkKeyNotVerifiedError(t *testing.T, testName string, methodName string, err error) {
	if err == nil {
		// There's no error to check - throw an error and return
		t.Errorf("%s fail: %s succeeded without a verified key", testName, methodName)
		return
	}
	if err != errKeyNotVerified {
		t.Errorf("%s fail: %s returned unexpected error %s", testName, methodName, err.Error())
	}
}

func checkMissingFieldError(t *testing.T, testName string, methodName string, err error, fieldNames ...string) {
	if err == nil {
		// There's no error to check - throw an error and return
		t.Errorf("%s fail: %s succeeded with missing fields %s", testName, methodName, strings.Join(fieldNames, ", "))
		return
	}

	e, ok := err.(APIError)
	if !ok {
		// We don't have the right type of error to check - throw an error and return
		t.Errorf("%s fail: %s returned '%s', expected 'Missing required field: %s.'", testName, methodName, err.Error(), strings.Join(fieldNames, " or "))
		return
	}

	for _, field := range fieldNames {
		if e.Result == "Missing required field: "+field+"." {
			// Success! We got the error message we expected - return with no error
			return
		}
	}

	t.Errorf("%s fail: %s returned '%s', expected 'Missing required field: %s.'", testName, methodName, e.Result, strings.Join(fieldNames, " or "))
}

func checkSpamStatus(t *testing.T, testName string, expectedStatus SpamStatus, status SpamStatus, err error) {
	if err != nil {
		// We don't have a spam status to check - throw an error and return
		t.Errorf("%s fail: %s", testName, err.Error())
		return
	}
	if status != expectedStatus {
		t.Errorf("%s fail: Status %s, Expected %s", testName, status, expectedStatus)
	}
}

func TestAPIKeyNotVerified(t *testing.T) {
	params := url.Values{}

	// CheckComment should fail if the API key isn't verified
	_, err := api.CheckComment(&params)
	checkKeyNotVerifiedError(t, "APIKeyNotVerified", "CheckComment", err)

	// SubmitSpam should fail if the API key isn't verified
	err = api.SubmitSpam(&params)
	checkKeyNotVerifiedError(t, "APIKeyNotVerified", "SubmitSpam", err)

	// SubmitHam should fail if the API key isn't verified
	err = api.SubmitHam(&params)
	checkKeyNotVerifiedError(t, "APIKeyNotVerified", "SubmitHam", err)
}

// Test key verification
func TestAPIVerifyKey(t *testing.T) {
	err := api.VerifyKey(config.APIKey, config.Site)
	if err != nil {
		t.Errorf("APIVerifyKey %s fail: %s", config.APIKey, err.Error())
	}
}

// To simulate a positive (spam) result, make a comment-check API call
// with the comment_author set to viagra-test-123 and all other required
// fields populated with typical values.
func TestAPICheckSpam(t *testing.T) {

	// Set up params for a positive result
	params := url.Values{
		_Site:      {config.Site},
		_UserIP:    {config.IP},
		_UserAgent: {config.UserAgent},
		_Author:    {"viagra-test-123"},
	}

	// And test them
	status, err := api.CheckComment(&params)
	checkSpamStatus(t, "APICheckSpam", StatusProbableSpam, status, err)

	// Add the discard flag to simulate blatant or "pervasive" spam
	params.Set("test_discard", "true")

	// And test again
	status, err = api.CheckComment(&params)
	checkSpamStatus(t, "APICheckSpam", StatusDefiniteSpam, status, err)

	// Check that the test mode parameter is being cleaned up
	checkTestModeCleanup(t, "APICheckSpam", &params)
}

// To simulate a negative (not spam) result, make a comment-check API call
// with the user_role set to administrator and all other required fields
// populated with typical values.
func TestAPICheckHam(t *testing.T) {

	// Set up params for a negative result
	params := url.Values{
		_Site:       {config.Site},
		_UserIP:     {config.IP},
		_UserAgent:  {config.UserAgent},
		"user_role": {"administrator"},
	}

	// And test them
	status, err := api.CheckComment(&params)
	checkSpamStatus(t, "APICheckHam", StatusNotSpam, status, err)

	// Check that the test mode parameter is being cleaned up
	checkTestModeCleanup(t, "APICheckHam", &params)
}

// According to the Akismet docs, blog, user_ip and user_agent are required
// parameters for comment-check. In practice only the first two are actually
// required. API calls without a user agent will succeed.
//
// This is more a test of the Akismet API than a test of the gokismet code.
// It's really only here to flag any changes in the API behaviour.
func TestAPICheckRequired(t *testing.T) {

	var err error
	var params url.Values

	// Missing: blog, ip
	// Should throw an error
	params = defaultParams()
	params.Del(_Site)
	params.Del(_UserIP)

	_, err = api.CheckComment(&params)
	checkMissingFieldError(t, "APICheckRequired", "CheckComment", err, _Site, _UserIP)

	// Missing: blog
	// Should throw an error
	params = defaultParams()
	params.Del(_Site)

	_, err = api.CheckComment(&params)
	checkMissingFieldError(t, "APICheckRequired", "CheckComment", err, _Site)

	// Missing: ip
	// Should throw an error
	params = defaultParams()
	params.Del(_UserIP)

	_, err = api.CheckComment(&params)
	checkMissingFieldError(t, "APICheckRequired", "CheckComment", err, _UserIP)

	// Missing: user agent
	// Should NOT throw an error
	params = defaultParams()
	params.Del(_UserAgent)

	_, err = api.CheckComment(&params)
	if err != nil {
		t.Errorf("APICheckRequired fail: CheckComment failed with missing fields: %s", _UserAgent)
	}
}

func TestAPISubmitSpam(t *testing.T) {
	params := defaultParams()
	err := api.SubmitSpam(&params)
	if err != nil {
		t.Errorf("APISubmitSpam fail: %s", err.Error())
	}

	// TODO: Come up with a failing test for api.SubmitSpam. It seems
	// to work even with no parameters.
	err = api.SubmitSpam(&url.Values{})
	if err != nil {
		t.Errorf("APISubmitSpam (no params) fail: %s", err.Error())
	}

	// Check that the test mode parameter is being cleaned up
	checkTestModeCleanup(t, "APISubmitSpam", &params)
}

func TestAPISubmitHam(t *testing.T) {
	params := defaultParams()
	err := api.SubmitHam(&params)
	if err != nil {
		t.Errorf("APISubmitHam fail: %s", err.Error())
	}

	// TODO: Come up with a failing test for api.SubmitHam. It seems
	// to work even with no parameters.
	err = api.SubmitHam(&url.Values{})
	if err != nil {
		t.Errorf("APISubmitHam (no params) fail: %s", err.Error())
	}

	// Check that the test mode parameter is being cleaned up
	checkTestModeCleanup(t, "APISubmitHam", &params)
}

func TestCommentNew(t *testing.T) {
	var err error
	// Note: If we use := here, Go will create a local Comment
	// object instead of initialising the global.
	comment, err = NewTestComment(config.APIKey, config.Site)
	if err != nil {
		// We can't continue without a Comment object
		t.Fatalf("CommentNew fail: %s", err.Error())
	}
	//comment.DebugTo(os.Stdout)
}

func TestCommentCheck(t *testing.T) {
	if comment == nil {
		// We can't continue without a Comment object
		t.Fatal("CommentCheck fail: Comment object is nil.")
	}

	// Set up a non-spam comment
	comment.SetUserIP(config.IP)
	comment.SetUserAgent(config.UserAgent)
	comment.SetReferer("http://www.google.com")
	comment.SetPage(config.Article)
	// Set article timestamp to 1 month ago
	comment.SetPageTimestamp(time.Now().AddDate(0, -1, 0))
	comment.SetAuthor("gokismet test")
	comment.SetEmail("hello@example.com")
	comment.SetContent("This is an example comment that does not contain anything spammy. In the absence of other dodgy settings Akismet should return a negative (non-spam) response...")
	comment.SetURL("http://www.example.com")
	// Set comment timestamp to current time
	comment.SetTimestamp(time.Now())
	comment.SetSiteLanguage("en_us")
	comment.SetCharset("UTF-8")

	// And test it
	status, err := comment.Check()
	checkSpamStatus(t, "CommentCheck", StatusNotSpam, status, err)

	// Make the comment spammy
	comment.SetAuthor("viagra-test-123")

	// And test again
	status, err = comment.Check()
	checkSpamStatus(t, "CommentCheck", StatusProbableSpam, status, err)

	// Make the comment non-spammy again
	comment.SetAuthor("gokismet-test")

	// And test a final time
	status, err = comment.Check()
	checkSpamStatus(t, "CommentCheck", StatusNotSpam, status, err)
}

func TestCommentReport(t *testing.T) {
	if comment == nil {
		// We can't continue without a Comment object
		t.Fatal("CommentReport fail: Comment object is nil.")
	}

	if err := comment.ReportSpam(); err != nil {
		t.Errorf("CommentReport Spam fail: %s", err.Error())
	}

	if err := comment.ReportNotSpam(); err != nil {
		t.Errorf("CommentReport NotSpam fail: %s", err.Error())
	}
}
