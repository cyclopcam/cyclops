package org.cyclops;

import android.webkit.CookieManager;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

import okhttp3.Cookie;
import okhttp3.CookieJar;
import okhttp3.HttpUrl;

// One possibly important note from this vibe-coded class:
// Ensure your WebView has had a chance to load a page or that cookies have been set by the WebView
// before you expect OkHttpClient to pick them up. If the WebsocketPlayer is created very early in
// the app lifecycle, before any WebView activity, the CookieManager might be empty.
public class WebViewCookieJar implements CookieJar {

    private final CookieManager webViewCookieManager = CookieManager.getInstance();

    @Override
    public void saveFromResponse(HttpUrl url, List<Cookie> cookies) {
        String urlString = url.toString();
        for (Cookie cookie : cookies) {
            webViewCookieManager.setCookie(urlString, cookie.toString());
        }
        // Ensure cookies are flushed to storage if necessary
        // CookieManager.getInstance().flush(); // Usually not needed immediately, system handles it.
    }

    @Override
    public List<Cookie> loadForRequest(HttpUrl url) {
        String urlString = url.toString();
        String cookiesString = webViewCookieManager.getCookie(urlString);

        if (cookiesString != null && !cookiesString.isEmpty()) {
            String[] cookieHeaders = cookiesString.split(";");
            List<Cookie> cookies = new ArrayList<>(cookieHeaders.length);
            for (String header : cookieHeaders) {
                Cookie parsedCookie = Cookie.parse(url, header);
                if (parsedCookie != null) {
                    cookies.add(parsedCookie);
                }
            }
            return cookies;
        }
        return Collections.emptyList();
    }
}
