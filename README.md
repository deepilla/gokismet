# Gokismet

[![GoDoc](https://godoc.org/github.com/deepilla/gokismet?status.svg)](https://godoc.org/github.com/deepilla/gokismet)
[![Build Status](https://travis-ci.org/deepilla/gokismet.svg?branch=master)](https://travis-ci.org/deepilla/gokismet)

Gokismet is a Go library for the [Akismet](https://akismet.com/) anti-spam service. Use it to:

1. Check comments, forum posts, and other user-generated content for spam.

2. Notify Akismet of false positives (legitimate comments incorrectly flagged
as spam) and false negatives (spam incorrectly flagged as legitimate comments).

## Documentation

See [gokismet on GoDoc](https://godoc.org/github.com/deepilla/gokismet) for detailed docs on this library.

For background on Akismet, see:

- [Akismet API docs](https://akismet.com/development/api/#detailed-docs)
- [Types of spam in Akismet](https://blog.akismet.com/2014/04/23/theres-a-ninja-in-your-akismet/ "There's a ninja in your Akismet")
- [Akismet comment types](https://blog.akismet.com/2012/06/19/pro-tip-tell-us-your-comment_type/ "Pro Tip: Tell us your comment type")

## Installation

    go get github.com/deepilla/gokismet

## Usage

Import the gokismet package.

``` go
import "github.com/deepilla/gokismet"
```

#### Checking for spam

To check a comment for spam, call `NewAPI` to create an instance of the `API` type. Then call its `CheckComment` method, passing in the comment data as a map of key-value pairs.

```go
api := gokismet.NewAPI("YOUR_API_KEY", "http://your.website.com")

values := map[string]string{
    // Comment data goes here...
}

status, err := api.CheckComment(values)
```

Gokismet provides a `Comment` type to generate the key-value pairs. Define a `Comment` with the required fields, then call its `Values` method to extract the key-value pairs.

```go
comment := gokismet.Comment{
    UserIP:        "127.0.0.1",
    UserAgent:     "Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6",
    Page:          "http://your.website.com/2016/05/05/its-cinco-de-mayo/",
    PageTimestamp: time.Date(2016, time.May, 5, 10, 30, 0, 0, time.UTC),
    Author:        "A. Commenter",
    AuthorEmail:   "acommenter@aol.com",
    Content:       "I love Cinco de Mayo!",
    // etc...
}

status, err := api.CheckComment(comment.Values())
```

#### Reporting errors

If `CheckComment` flags a legitimate comment as spam (or vice versa), report the error to Akismet using the `API` method `SubmitHam` (or `SubmitSpam`). The steps are the same as for a spam check.

```go
api := gokismet.NewAPI("YOUR_API_KEY", "http://your.website.com")

comment := gokismet.Comment{
    // Set comment fields here...
}

err := api.SubmitHam(comment.Values())
```

## Licensing

Gokismet is provided under an [MIT License](http://choosealicense.com/licenses/mit/). See the [LICENSE](LICENSE) file for details.
