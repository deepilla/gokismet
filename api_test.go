package gokismet

import (
	"encoding/json"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
)

// Error codes for the initConfig function
const (
	// No config file
	codeMissingConfig = 100 + iota
	// Couldn't parse the JSON in the config file
	codeInvalidConfig
	// Config file does not include the required entries
	codeIncompleteConfig
	// Couldn't parse the website in the config file
	codeInvalidSite
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

var api API         // API object used in tests
var config settings // Configuration settings, read from a JSON file

func TestMain(m *testing.M) {

	// Read config settings
	errno := initConfig()
	if errno != 0 {
		os.Exit(errno)
	}

	// Initialise global API object
	initAPI()

	// Run the jewels...
	os.Exit(m.Run())
}

func initAPI() {
	api.TestMode = true
	api.UserAgent = "Gokismet.API"
	if config.Debug {
		api.Output = os.Stdout
	}
}

func initConfig() int {
	// Open the settings file
	f, err := os.Open("testconfig.json")
	if err != nil {
		return codeMissingConfig
	}
	defer f.Close()
	// Use the contents to populate the settings struct
	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return codeInvalidConfig
	}
	// Check that the settings have been populated correctly
	if config.APIKey == "" || config.Site == "" || config.IP == "" || config.UserAgent == "" {
		return codeIncompleteConfig
	}
	// If there's no article URL specified in the config, make one up
	if config.Article == "" {
		u, err := url.Parse(config.Site)
		if err != nil {
			return codeInvalidSite
		}
		u.Path = "blog/"
		u.RawQuery = "p=123"

		config.Article = u.String()
	}

	return 0
}

// Return the name of a function in the current stack. Skip is the
// number of stack frames to ascend, where 0 means the current function
// (i.e. getFunctionName()), 1 means the calling function, and so on.
func getFunctionName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "InvalidCaller"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "UnknownFunction"
	}

	s := strings.Split(fn.Name(), ".")
	return s[len(s)-1]
}

// Non-spammy comment parameters, used in various tests
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

// Our global API object is in test mode. An is_test flag is added to the
// query parameters for any method call. Check that the flag is being
// cleaned up afterwards (i.e. deleted from the query parameters).
func assertParamsDoNotContainTestFlag(t *testing.T, method string, params *url.Values) {
	if params.Get("is_test") != "" {
		t.Errorf("%s fail: Test Mode flag was not removed after calling %s", getFunctionName(2), method)
	}
}

// The main API methods will not work unless an Akismet API key has been
// verified. Check that those methods fail before verification. We should
// get an errKeyNotVerified error.
func assertMethodFailsBeforeVerify(t *testing.T, method string, err error) {
	if err == nil {
		t.Errorf("%s fail: %s succeeded without a verified key", getFunctionName(2), method)
		return
	}
	if err != errKeyNotVerified {
		t.Errorf("%s fail: %s returned '%s', expected '%s'", getFunctionName(2), method, err, errKeyNotVerified)
	}
}

// If an Akismet API call has required parameters, check that the
// corresponding gokismet.API method fails without those parameters.
// We should get a "Missing required field" APIError error.
func assertMethodFailsWithoutRequiredField(t *testing.T, method string, err error, fields ...string) {
	if err == nil {
		t.Errorf("%s fail: %s succeeded with missing fields %s",
			getFunctionName(2),
			method,
			strings.Join(fields, ", "),
		)
		return
	}

	if e, ok := err.(APIError); ok {
		for _, field := range fields {
			if e.Result == "Missing required field: "+field+"." {
				// Success! We got the error message we expected - return with no error
				return
			}
		}
	}

	t.Errorf("%s fail: %s returned '%s', expected 'Missing required field: %s.'",
		getFunctionName(2),
		method,
		err,
		strings.Join(fields, " or "),
	)
}

// Check that a spam check returned the spam status we expect.
func assertSpamStatusEquals(t *testing.T, method string, expected, status SpamStatus, err error) {
	if err != nil {
		t.Errorf("%s fail: %s returned error '%s', expected status '%s'",
			getFunctionName(2),
			method,
			err,
			expected,
		)
		return
	}
	if status != expected {
		t.Errorf("%s fail: %s returned status '%s', expected '%s'",
			getFunctionName(2),
			method,
			status,
			expected,
		)
	}
}

// Confirm that API methods fail without a verified API key
func TestKeyNotVerified(t *testing.T) {
	params := defaultParams()

	// CheckComment should fail if the API key isn't verified
	_, err := api.CheckComment(&params)
	assertMethodFailsBeforeVerify(t, "API.CheckComment", err)

	// SubmitSpam should fail if the API key isn't verified
	err = api.SubmitSpam(&params)
	assertMethodFailsBeforeVerify(t, "API.SubmitSpam", err)

	// SubmitHam should fail if the API key isn't verified
	err = api.SubmitHam(&params)
	assertMethodFailsBeforeVerify(t, "API.SubmitHam", err)
}

// Confirm key verification/storage
func TestVerifyKey(t *testing.T) {
	err := api.VerifyKey(config.APIKey, config.Site)
	if err != nil {
		t.Errorf("%s fail: verify '%s' returned '%s'", getFunctionName(1), config.APIKey, err)
	}
	if api.key != config.APIKey {
		t.Errorf("%s fail: api key '%s' has not been stored", getFunctionName(1), config.APIKey)
	}
}

// Confirm spam detection
// From the Akismet docs: To simulate a positive (spam) result, make a
// comment-check API call with the comment_author set to viagra-test-123
// and all other required fields populated with typical values.
func TestDetectSpam(t *testing.T) {

	// Set up params for a positive result
	params := url.Values{
		_Site:      {config.Site},
		_UserIP:    {config.IP},
		_UserAgent: {config.UserAgent},
		_Author:    {"viagra-test-123"},
	}

	// And test them
	status, err := api.CheckComment(&params)
	assertSpamStatusEquals(t, "API.CheckComment", StatusProbableSpam, status, err)

	// Add the discard flag to simulate blatant or "pervasive" spam
	params.Set("test_discard", "true")

	// And test again
	status, err = api.CheckComment(&params)
	assertSpamStatusEquals(t, "API.CheckComment", StatusDefiniteSpam, status, err)

	// Check that the test mode parameter is being cleaned up
	assertParamsDoNotContainTestFlag(t, "API.CheckComment", &params)
}

// Confirm ham detection
// From the Akismet docs: To simulate a negative (not spam) result, make a
// comment-check API call with the user_role set to administrator and all
// other required fields populated with typical values.
func TestDetectHam(t *testing.T) {

	// Set up params for a negative result
	params := url.Values{
		_Site:       {config.Site},
		_UserIP:     {config.IP},
		_UserAgent:  {config.UserAgent},
		"user_role": {"administrator"},
	}

	// And test them
	status, err := api.CheckComment(&params)
	assertSpamStatusEquals(t, "API.CheckComment", StatusNotSpam, status, err)

	// Check that the test mode parameter is being cleaned up
	assertParamsDoNotContainTestFlag(t, "API.CheckComment", &params)
}

// According to the Akismet docs, blog and user_ip are required parameters
// for comment-check and user_agent is recommended but not required. Confirm
// that API.CheckComment reflects this behaviour. Unless the Akismet API changes
// this test should always pass.
func TestRequiredFields(t *testing.T) {

	var err error
	var params url.Values

	// Missing: blog, ip
	// Should throw an error
	params = defaultParams()
	params.Del(_Site)
	params.Del(_UserIP)

	_, err = api.CheckComment(&params)
	assertMethodFailsWithoutRequiredField(t, "API.CheckComment", err, _Site, _UserIP)

	// Missing: blog
	// Should throw an error
	params = defaultParams()
	params.Del(_Site)

	_, err = api.CheckComment(&params)
	assertMethodFailsWithoutRequiredField(t, "API.CheckComment", err, _Site)

	// Missing: ip
	// Should throw an error
	params = defaultParams()
	params.Del(_UserIP)

	_, err = api.CheckComment(&params)
	assertMethodFailsWithoutRequiredField(t, "API.CheckComment", err, _UserIP)

	// Missing: user agent
	// Should NOT throw an error
	params = defaultParams()
	params.Del(_UserAgent)

	_, err = api.CheckComment(&params)
	if err != nil {
		t.Errorf("%s fail: API.CheckComment failed with missing field: %s, returned '%s'",
			getFunctionName(1),
			_UserAgent,
			err,
		)
	}
}

// Confirm spam/ham submission.
// TODO: We need failing tests for api.SubmitSpam and api.SubmitHam. It's
// impossible to tell if the calls are working if they always return success.
func TestSubmit(t *testing.T) {
	params := defaultParams()

	if err := api.SubmitSpam(&params); err != nil {
		t.Errorf("%s fail: API.SubmitSpam returned '%s'", getFunctionName(1), err)
	}

	if err := api.SubmitHam(&params); err != nil {
		t.Errorf("%s fail: API.SubmitHam returned '%s'", getFunctionName(1), err)
	}

	// Check that the test mode parameter is being cleaned up
	assertParamsDoNotContainTestFlag(t, "API.SubmitSpam", &params)
}

// Check spam/ham submission with an empty request body.
// Akismet's submit-spam and submit-ham methods seemingly never fail, even
// when passed an empty parameter set. See Test_Submit re failing tests
// for the submit methods.
func TestEmptySubmit(t *testing.T) {

	// Exit test mode so we have a completely empty request body.
	// Not even the is_test parameter is set.
	api.TestMode = false

	if err := api.SubmitSpam(&url.Values{}); err != nil {
		t.Errorf("%s fail: API.SubmitSpam (no params) returned '%s'", getFunctionName(1), err)
	}

	if err := api.SubmitHam(&url.Values{}); err != nil {
		t.Errorf("%s fail: API.SubmitHam (no params) returned '%s'", getFunctionName(1), err)
	}
}
