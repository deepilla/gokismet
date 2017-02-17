# Gokismet

[![GoDoc](https://godoc.org/github.com/deepilla/gokismet?status.svg)](https://godoc.org/github.com/deepilla/gokismet)
[![Build Status](https://travis-ci.org/deepilla/gokismet.svg?branch=master)](https://travis-ci.org/deepilla/gokismet)
[![Go Report Card](https://goreportcard.com/badge/github.com/deepilla/gokismet)](https://goreportcard.com/report/github.com/deepilla/gokismet)

Gokismet is a Go library for the [Akismet](https://akismet.com/) anti-spam service.

Use gokismet to:

1. Check comments, forum posts, and other user-generated content for spam.

2. Notify Akismet of false positives (legitimate content incorrectly flagged
as spam) and false negatives (spam content that it failed to detect).

## Documentation

See [gokismet on GoDoc](https://godoc.org/github.com/deepilla/gokismet) for detailed docs on this library.

For background on Akismet, see:

- [Akismet API docs](https://akismet.com/development/api/#detailed-docs)
- [The two types of spam in Akismet](https://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/ "There's a ninja in your Akismet")
- [Comment types in Akismet](https://blog.akismet.com/2012/06/19/pro-tip-tell-us-your-comment_type/ "Pro Tip: Tell us your comment type")

## Installation

    go get github.com/deepilla/gokismet

## Usage

Import the gokismet package.

``` go
import "github.com/deepilla/gokismet"
```

### Checking for spam

To check content for spam, call the `NewChecker` function to create an instance of the `Checker` type. Then call its `Check` method, passing in the content as a map of key-value pairs.

```go
ch := gokismet.NewChecker("YOUR-API-KEY", "http://example.com")

comment := gokismet.Comment{
    UserIP:        "127.0.0.1",
    UserAgent:     "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
    Page:          "http://example.com/posts/feliz-cinco-de-mayo/",
    PageTimestamp: time.Date(2016, time.May, 5, 10, 30, 0, 0, time.UTC),
    Author:        "A. Commenter",
    AuthorEmail:   "acommenter@aol.com",
    Content:       "I love Cinco de Mayo!",
    // etc...
}

status, err := ch.Check(comment.Values())

switch status {
case gokismet.StatusHam:
    fmt.Println("Comment is legit")
case gokismet.StatusProbableSpam, gokismet.StatusDefiniteSpam:
    fmt.Println("Comment is spam")
case gokismet.StatusUnknown:
    fmt.Println("Something went wrong:", err)
}
```

**Note**: The `Comment` type is optional. You can also declare your content as a map of strings to strings. But using a `Comment` is more convenient as you don't have to know the Akismet key names.

### Reporting errors

If `Check` flags some legitimate content as spam or misses some spam, you can report the error to Akismet using the Checker's `ReportHam` or `ReportSpam` methods. The steps are the same as for a spam check.

## Licensing

Gokismet is provided under an [MIT License](http://choosealicense.com/licenses/mit/). See the [LICENSE](LICENSE) file for details.
