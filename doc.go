/*
Package gokismet is a Go implementation of the Akismet anti-spam API.
See http://akismet.com/development/api/#detailed-docs for the Akismet
documentation. You will need an Akismet API key to use this library.

Gokismet provides two main classes:

1. API is a thin wrapper around Akismet's REST API. For most situations,
you won't need to use this class directly.

2. Comment is a higher-level class built on top of API. It includes helper
functions for the most common Akismet use-cases: blog comments, forum
posts, contact forms and so on.

Example 1. Checking spam with the Comment class:

    comment, err := gokismet.NewComment("YOUR_API_KEY", "http://www.yourwebsite.com")
    if err != nil {
        // Handle the error
    }

    comment.SetUserIP("127.0.0.1")
    comment.SetUserAgent("Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6")
    comment.SetAuthor("A. Commenter")
    comment.SetEmail("acommenter@aol.com")
    comment.SetURL("http://www.lovecincodemayo.com")
    comment.SetPermalink("http://www.yourwebsite.com/2015/05/05/its-cinco-de-mayo/")
    comment.SetContent("I love Cinco de Mayo!")
    ...

    status, err := comment.Check()
    if err != nil {
        // Handle the error
    }

    switch status {
        case gokismet.StatusNotSpam:
            fmt.Println("Akismet thinks this is a legit comment")
        case gokismet.StatusProbableSpam:
            fmt.Println("Akismet thinks this is spam")
        case gokismet.StatusDefiniteSpam:
            fmt.Println("Akismet thinks this is the worst kind of spam")
    }

Example 2. Checking spam with the API class:

    params := url.Values{
        "blog",                 {"http://yourwebsite.com"},
        "user_ip",              {"127.0.0.1"},
        "user_agent",           {"Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6"},
        "comment_author",       {"admin"},
        "comment_author_email", {"test@test.com"},
        "comment_author_url",   {"http://www.CheckOutMyCoolSite.com"},
        "permalink",            {"http://yourwebsite.com/blog/post=1"},
        "comment_type",         {"comment"},
        "comment_content",      {"This is the comment text."},
        ...
    }

    api := NewAPI()

    err := api.VerifyKey("YOUR_API_KEY", "http://yourwebsite.com")
    if err != nil {
        // Handle the error
    }

    status, err := api.CheckComment(&params)
    if err != nil {
        // Handle the error
    }

    switch status {
        case gokismet.StatusNotSpam:
            fmt.Println("Akismet thinks this is a legit comment")
        case gokismet.StatusProbableSpam:
            fmt.Println("Akismet thinks this is spam")
        case gokismet.StatusDefiniteSpam:
            fmt.Println("Akismet thinks this is the worst kind of spam")
    }
*/
package gokismet
