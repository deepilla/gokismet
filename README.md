# Gokismet

[![GoDoc](https://godoc.org/github.com/deepilla/gokismet?status.svg)](https://godoc.org/github.com/deepilla/gokismet)

Gokismet is a Go implementation of the [Akismet anti-spam API](http://akismet.com/development/api/#detailed-docs). It allows you to:

- Check comments, forum posts, and other user-generated content for spam
- Report missed spam and incorrectly flagged spam to Akismet

**Note**: You will need an [Akismet API key](https://akismet.com/signup/?connect=yes&plan=developer) to use Gokismet.

## Documentation

Hosted on [GoDoc](https://godoc.org/github.com/deepilla/gokismet).

## Installation

``` go
go get github.com/deepilla/gokismet
```

## Usage

Import the gokismet package.

``` go
import "github.com/deepilla/gokismet"
```

### Checking for spam

Checking comments for spam is a simple 3-step process.

Step 1: Create a new `Comment` object.

```go
comment, err := gokismet.NewComment("YOUR_API_KEY", "http://www.yourwebsite.com")
if err != nil {
    // Handle the error
}
```

Step 2: Set your comment data.

```go
comment.SetUserIP("127.0.0.1")
comment.SetUserAgent("Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.2) Gecko/20100115 Firefox/3.6")
comment.SetPermalink("http://www.yourwebsite.com/2015/05/05/its-cinco-de-mayo/")
comment.SetAuthor("A. Commenter")
comment.SetEmail("acommenter@aol.com")
comment.SetURL("http://www.lovecincodemayo.com")
comment.SetContent("I love Cinco de Mayo!")
...
```

Step 3: Call the `Check` function.

```go
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

### Reporting mistakes to Akismet

To notify Akismet of spam it failed to catch or legitimate content incorrectly
flagged as spam, follow steps 1 and 2 above, then call `MarkSpam` or `MarkNotSpam`.

## Testing

Gokismet's tests require an Akismet API key to pass. If you want to run the tests yourself you must provide your own API key in a JSON file named `testconfig.json` (along with a few other settings). The file should look something like this:

``` json
{
    "APIKey": "YOUR_API_KEY",
    "Site": "http://yourwebsite.com",
    "IP": "127.0.0.1",
    "UserAgent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36"
}
```

Save `testconfig.json` to the main project directory and you should be good to go. A [gitignore](.gitignore) entry ensures that your private API key isn't accidentally committed to a public repo.

## Licensing

[MIT License](http://choosealicense.com/licenses/mit/). See the [LICENSE](LICENSE) file for details.
