# Gokismet

Gokismet is a Go implementation of the [Akismet anti-spam API](http://akismet.com/development/api/#detailed-docs). It requires an [Akismet API key](https://akismet.com/signup/?connect=yes&plan=developer).

## Documentation

[![GoDoc](https://godoc.org/github.com/deepilla/gokismet?status.svg)](https://godoc.org/github.com/deepilla/gokismet)

Documentation for this package is [hosted on godoc.org](https://godoc.org/github.com/deepilla/gokismet).

## Usage

```go
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
```

## Testing

In order to run the tests for this package you need to create a JSON file named `testconfig.json` in the main project directory. This file contains configuration settings for the tests (including your Akismet API key). It should look something like this:

``` json
{
    "APIKey": "YOUR_API_KEY",
    "Site": "http://yourwebsite.com",
    "IP": "127.0.0.1",
    "UserAgent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36"
}
```

The [gitignore](.gitignore) file ensures that the config file is not accidentally committed to a public repo, exposing your private API key.

## Licensing

Gokismet is available under an [MIT License](http://choosealicense.com/licenses/mit/). See the [LICENSE](LICENSE) file for details.
