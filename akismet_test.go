// NOTE:
// The tests in this file make actual calls to the Akismet API.
// As such, they're only executed if an API key and website are
// provided on the command line.
//
// In order to ensure repeatable tests, the comment data contains
// special parameters that elicit specific responses from Akismet.
// All calls set the Akismet test flag (even if it's not clear
// what this actually does!).
//
// See https://akismet.com/development/api/#detailed-docs for info
// on these special paramters.

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

	data := []TestAPIData{
		{
			// Setting user_role to "administrator" gives
			// a negative spam response.
			Params: map[string]string{
				"is_test":   "true",
				"user_role": "administrator",
			},
			Status: gokismet.StatusHam,
		},
		{
			// Setting comment_author to "viagra-test-123" gives
			// a positive spam response.
			Params: map[string]string{
				"is_test":        "true",
				"comment_author": "viagra-test-123",
			},
			Status: gokismet.StatusProbableSpam,
		},
		{
			// Adding "test_discard" gives a "pervasive" spam
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

	// Akismet's submit-ham and submit-spam methods
	// only have one valid response.
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

	// Define our test comment data. These values come from
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

	api := gokismet.NewAPI(*fAPIKey, *fSite)

	for i, test := range data {

		values := comment.Values()
		// Add any special parameters to our comment data.
		for k, v := range test.Params {
			values[k] = v
		}

		// Call the API.
		status, err := fn(api, values)

		// Check the returned spam status and error.
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
