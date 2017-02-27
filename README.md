# Gokismet

[![GoDoc](https://godoc.org/github.com/deepilla/gokismet?status.svg)](https://godoc.org/github.com/deepilla/gokismet)
[![Build Status](https://travis-ci.org/deepilla/gokismet.svg?branch=master)](https://travis-ci.org/deepilla/gokismet)
[![Go Report Card](https://goreportcard.com/badge/github.com/deepilla/gokismet)](https://goreportcard.com/report/github.com/deepilla/gokismet)

Gokismet is a Go library for the [Akismet](https://akismet.com/) anti-spam service.

Use gokismet to:

1. Check comments, forum posts, and other user-generated content for spam.

2. Notify Akismet of false positives (legitimate content incorrectly flagged
as spam) and false negatives (spam that it failed to detect).

## Installation

    go get github.com/deepilla/gokismet

## Usage

Import the gokismet package.

``` go
import "github.com/deepilla/gokismet"
```

### Checking for spam

To check content for spam, first call the `NewChecker` function to create an instance of the `Checker` type. Then call its `Check` method, passing in the content as a map of key-value pairs.

```go
// Define your content. This example uses the Comment
// type to generate its key-value pairs but you can
// also build the map manually (see the Akismet docs
// for the list of valid keys).
comment := gokismet.Comment{
    UserIP:        "127.0.0.1",
    UserAgent:     "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
    Page:          "http://your-website.com/posts/feliz-cinco-de-mayo/",
    PageTimestamp: time.Date(2016, time.May, 5, 10, 30, 0, 0, time.UTC),
    Author:        "A. Commenter",
    AuthorEmail:   "acommenter@aol.com",
    Content:       "I love Cinco de Mayo!",
    // etc...
}

// Create a Checker, specifying your Akismet API key
// and the homepage URL of your website.
ch := gokismet.NewChecker("YOUR-API-KEY", "http://your-website.com")

// Call the Check method, passing in your content.
status, err := ch.Check(comment.Values())

// Handle the results.
switch status {
case gokismet.StatusHam:
    fmt.Println("This is legit content")
case gokismet.StatusProbableSpam, gokismet.StatusDefiniteSpam:
    fmt.Println("This is spam")
case gokismet.StatusUnknown:
    fmt.Println("Something went wrong:", err)
}
```

### Reporting errors

Akismet may occasionally get things wrong, either by flagging legitimate content as spam or failing to identify spam. You can report these errors to Akismet with the `ReportHam` and `ReportSpam` methods.

The process is the same as for the `Check` method: create a `Checker`, then call the relevant method, passing in the content as key-value pairs.

## Further Reading

For detailed documentation on this package, see [gokismet on GoDoc](https://godoc.org/github.com/deepilla/gokismet).

For background on Akismet, see:

- [Akismet API docs](https://akismet.com/development/api/#detailed-docs)
- [Types of spam in Akismet](https://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/)
- [Comment types in Akismet](https://blog.akismet.com/2012/06/19/pro-tip-tell-us-your-comment_type/)

## Licensing

Gokismet is provided under an [MIT License](http://choosealicense.com/licenses/mit/). See the [LICENSE](LICENSE) file for details.
