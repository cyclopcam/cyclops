package org.cyclops;

import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.WebViewAssetLoader;

import android.annotation.SuppressLint;
import android.os.Bundle;
import android.util.Log;
import android.webkit.WebSettings;
import android.webkit.WebView;

public class MainActivity extends AppCompatActivity {
    WebView mWebView;
    LocalContentWebViewClient mLocalClient;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        State.global.scanner = new Scanner(this);

        mWebView = findViewById(R.id.webview);
        WebSettings settings = mWebView.getSettings();
        // Javascript is not enabled by default!
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);

        // dev time
        WebView.setWebContentsDebuggingEnabled(true);

        //setContentView(mWebView);

        // We need this in order to talk to a Cyclops server on the LAN..
        // Dammit even this doesn't work.
        //settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);

        // We might want to enable this full list: https://stackoverflow.com/questions/7548172/javascript-not-working-in-android-webview
        /*
        webSettings.setJavaScriptEnabled(true);
        webSettings.setDomStorageEnabled(true);
        webSettings.setLoadWithOverviewMode(true);
        webSettings.setUseWideViewPort(true);
        webSettings.setBuiltInZoomControls(true);
        webSettings.setDisplayZoomControls(false);
        webSettings.setSupportZoom(true);
        webSettings.setDefaultTextEncodingName("utf-8");
         */

        final WebViewAssetLoader assetLoader = new WebViewAssetLoader.Builder()
                .addPathHandler("/assets/", new WebViewAssetLoader.AssetsPathHandler(this))
                .addPathHandler("/res/", new WebViewAssetLoader.ResourcesPathHandler(this))
                .build();
        mLocalClient = new LocalContentWebViewClient(assetLoader);
        mWebView.setWebViewClient(mLocalClient);
        mWebView.loadUrl("https://appassets.androidplatform.net/assets/index.html");

        //mWebView.loadUrl("http://192.168.10.11:8080");
    }

    @Override
    public void onBackPressed() {
        mLocalClient.cyBack(mWebView, this);
    }

    public void webViewBackFailed() {
        // This will usually exit the activity
        Log.i("C", "going super.back");
        super.onBackPressed();
    }
}