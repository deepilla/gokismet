/*
Package gokismet is a Go implementation of the Akismet anti-spam API. It allows
you to check comments, forum posts, and other user-generated content for spam
and report missed spam or incorrectly flagged spam to Akismet.

Gokismet provides two classes:

1. API is a wrapper around Akismet's REST API. Typically you won't use
this directly.

2. Comment is a convenience class built on top of API. It provides helper
functions that hide the implementation details of the Akismet API.

Note

An Akismet API key is required to use this library.

Background

See http://akismet.com/development/api/#detailed-docs for the Akismet API docs.
*/
package gokismet
