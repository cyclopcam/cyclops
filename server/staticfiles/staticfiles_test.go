package staticfiles

import (
	"testing"
)

func TestStaticFileRegex(t *testing.T) {
	webpack := func(url string) {
		if !reWebpackAsset.MatchString(url) {
			t.Errorf("Expected %v to MATCH a webpack asset, but it does not", url)
		}
	}
	not := func(url string) {
		if reWebpackAsset.MatchString(url) {
			t.Errorf("Expected %v to NOT match a webpack asset, but it does", url)
		}
	}

	// Positive examples:
	// about.52e3024d.js
	// about.52e3024d.js.map
	// app.b8630bdd.js
	// app.b8630bdd.js.map
	// chunk-vendors.9c15f784.js
	// chunk-vendors.9c15f784.js.map
	// unittest.ad6c7e87.js
	// unittest.ad6c7e87.js.map

	// Negative examples:
	// favicon.ico
	// index.css
	// index.html

	webpack("/js/app.b8630bdd.js")
	webpack("/js/foo/x/y/z/about.52e3024d.js")
	webpack("about.52e3024d.js")
	webpack("about.52e3024d.js.map")
	webpack("app.b8630bdd.js")
	webpack("app.b8630bdd.js.map")
	webpack("chunk-vendors.9c15f784.js")
	webpack("chunk-vendors.9c15f784.js.map")
	webpack("unittest.ad6c7e87.js")
	webpack("unittest.ad6c7e87.js.map")
	not("app.js")
	not("/js/app.js")
	not("/js/app.ab.js")
	not("/js/app.1234.js")
	// 6 hex digits (not enough)
	not("/js/app.1234ab.js")
	// 7 hex digits (enough.. I don't know the precise formatting of the hash.. ie if it's %08X or %X)
	// Remember if the asset is determined to be "not hashed", then end up setting a conservative Must-Revalidate on the
	// cache header. So even if 1/10 builds we send Must-Revalidate.. then I guess that's OK. Sigh... what a messy PITA.
	webpack("/js/app.1234ab7.js")
	not("index.html")
}
