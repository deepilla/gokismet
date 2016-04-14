// NOTE:
// The tests in this file make actual calls to the Akismet API.
// As such, they're only executed if an API key and website are
// provided on the command line.
//
// In order to ensure repeatable tests, the comment data is
// supplemented with additional parameters that elicit specific
// responses from Akismet.
package gokismet_test

import (
	"flag"
	"path"
	"testing"

	"github.com/deepilla/gokismet"
)

// The Akismet API key and registered website are defined in
// command line flags.
// NOTE: We're relying on the standard "go test" command line
// parsing to populate these variables. Seems to work.
var (
	fAPIKey = flag.String("akismet.key", "", "an Akismet API Key")
	fSite   = flag.String("akismet.site", "", "website registered for the Akismet API Key")
)

// TestAPIData represents a test for the testAPI function.
type TestAPIData struct {
	Params map[string]string
	Status gokismet.SpamStatus
	Error  error
}

// TestAkismetCheckComment tests Akismet's responses to the
// API.CheckComment method.
func TestAkismetCheckComment(t *testing.T) {

	// See https://akismet.com/development/api/#detailed-docs for
	// info on these special paramter values.

	data := []TestAPIData{
		{
			// A user_role of "administrator" simulates a negative
			// spam response.
			Params: map[string]string{
				"is_test":   "true",
				"user_role": "administrator",
			},
			Status: gokismet.StatusHam,
		},
		{
			// A comment_author of "viagra-test-123" simulates a
			// positive spam response.
			Params: map[string]string{
				"is_test":        "true",
				"comment_author": "viagra-test-123",
			},
			Status: gokismet.StatusProbableSpam,
		},
		{
			// Adding "test_discard" simulates a "pervasive" spam
			// response.
			Params: map[string]string{
				"is_test":        "true",
				"comment_author": "viagra-test-123",
				"test_discard":   "true",
			},
			Status: gokismet.StatusDefiniteSpam,
		},
	}

	testAkismet(t, fnCheckComment, data)
}

// TestAkismetSubmitHam tests Akismet's responses to the
// API.SubmitHam method.
func TestAkismetSubmitHam(t *testing.T) {
	testAkismetSubmit(t, fnSubmitHam)
}

// TestAkismetSubmitSpam tests Akismet's responses to the
// API.SubmitSpam method.
func TestAkismetSubmitSpam(t *testing.T) {
	testAkismetSubmit(t, fnSubmitSpam)
}

// testAkismetSubmit implements TestAkismetSubmitHam and
// TestAkismetSubmitSpam.
func testAkismetSubmit(t *testing.T, submit ErrorFunc) {

	// NOTE: The submit API calls are extremely generous
	// about the parameters they accept. So there's only
	// one test scenario because I can't come up with a
	// failing case. I'm not even sure if the "is_test"
	// flag has any effect on submit calls but certainly
	// it can't do any harm.
	data := []TestAPIData{
		{
			Params: map[string]string{
				"is_test": "true",
			},
		},
	}

	testAkismet(t, toStatusErrorFunc(submit), data)
}

// testAkismet is a general function to test Akismet responses.
func testAkismet(t *testing.T, fn StatusErrorFunc, data []TestAPIData) {

	// Only run this test if we have an API key and website.
	if *fAPIKey == "" || *fSite == "" {
		t.SkipNow()
	}

	// Create some test comment data. These values come from
	// the Akismet API docs.
	comment := &gokismet.Comment{
		UserIP:      "127.0.0.1",
		UserAgent:   "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
		Referer:     "http://www.google.com",
		Page:        path.Join(*fSite, "blog/post=1"),
		Type:        "comment",
		Author:      "admin",
		AuthorEmail: "test@test.com",
		AuthorPage:  "http://www.CheckOutMyCoolSite.com",
		Content:     "It means a lot that you would take the time to review our software. Thanks again.",
	}

	// Create an API object.
	api := gokismet.NewAPI(*fAPIKey, *fSite)

	for i, test := range data {

		// Add any special values for this test to our standard
		// comment data.
		values := comment.Values()
		for k, v := range test.Params {
			values[k] = v
		}

		// Call the API.
		status, err := fn(api, values)

		// Test the returned spam status and error values.
		if status != test.Status {
			t.Errorf("Test %d: Expected Spam Status %q, got %q", i+1,
				spamStatusToString(test.Status), spamStatusToString(status))
		}

		errors := compareError(test.Error, err)
		for _, err := range errors {
			t.Errorf("Test %d: %s", i+1, err)
		}
	}
}
