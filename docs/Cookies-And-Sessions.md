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
