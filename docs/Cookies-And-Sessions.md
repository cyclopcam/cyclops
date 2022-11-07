# Cookies and Sessions

The classic approach for maintaining a login session in a browser app is to use
cookies. We stick with cookies because they have one very important benefit.
However, they also have a very important downside, which we discuss below.

## The benefit of cookies is:

**You do not need to write any code to include authentication data with every
request**

Without cookies, every API call ends up looking something like this:

`/api/protected/thing?auth=12345`

With cookies, you just do

`/api/protected/thing`

This permeates your entire app, and it's a nuisance to have to add it
everywhere.

## The downside of cookies is:

**They belong to a domain, so 192.168.1.14 on your home WiFi is
indistinguishable from 192.168.1.14 on your (adversarial) neighbour's WiFi.**

This is generally not a problem on the internet, where DNS and SSL protect us.
However, since we have a very clear goal of talking directly to devices over the
LAN, we must support connections over HTTP to a LAN IP address.

## The Workaround

In order to work around the above issue, we implement a public key verification
step into our native mobile app: Before opening a WebView into a LAN address, we
ask it to sign a challenge with its private key. If the signature that it
produces does not match the public key that we have on record for that IP
address, then we refuse to connect.

## Cookie Lifetime

Conceptually, we should be able to login once, acquire a cookie, and keep using
that cookie for years. However, this is not possible because Chrome's maximum
expiry time on a cookie is 400 days. So we need a mechanism to refresh our
session cookie. Our solution is to generate two tokens during initial login:

1. A bearer token
2. A session cookie

When the session cookie expires, or is invalid for any reason, we use our bearer
token to acquire a fresh cookie. The bearer token is stored in the native app's
database, and is only used when refreshing the cookie.
