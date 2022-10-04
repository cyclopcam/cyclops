package org.cyclops;

import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.WebViewAssetLoader;

import android.annotation.SuppressLint;
import android.app.ActionBar;
import android.os.Bundle;
import android.util.Log;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.LinearLayout;

import java.util.ArrayList;

public class MainActivity extends AppCompatActivity implements Main {
    WebView localWebView; // loads embedded JS code
    WebView remoteWebView; // loads remote JS code from a Cyclops server
    LocalContentWebViewClient localClient;
    RemoteWebViewClient remoteClient;
    boolean isRemote = false;
    ArrayList<String> history = new ArrayList<>();

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        State.global.scanner = new Scanner(this);
        State.global.connector = new Connector(this);

        // dev time
        WebView.setWebContentsDebuggingEnabled(true);

        localWebView = findViewById(R.id.localWebView);
        remoteWebView = findViewById(R.id.remoteWebView);
        setupWebView(localWebView);
        setupWebView(remoteWebView);

        final WebViewAssetLoader assetLoader = new WebViewAssetLoader.Builder()
                .addPathHandler("/assets/", new WebViewAssetLoader.AssetsPathHandler(this))
                .addPathHandler("/res/", new WebViewAssetLoader.ResourcesPathHandler(this))
                .build();
        localClient = new LocalContentWebViewClient(assetLoader, this);
        localWebView.setWebViewClient(localClient);
        localWebView.loadUrl("https://appassets.androidplatform.net/assets/index.html");

        remoteClient = new RemoteWebViewClient(this);
        remoteWebView.setWebViewClient(remoteClient);

        //openServer("http://192.168.10.15:8080");
    }

    @SuppressLint("SetJavaScriptEnabled")
    void setupWebView(WebView webview) {
        WebSettings settings = webview.getSettings();
        // Javascript is not enabled by default!
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);

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
    }

    @Override
    public void onBackPressed() {
        if (isRemote) {
            remoteClient.cyBack(remoteWebView);
        } else {
            localClient.cyBack(localWebView);
        }
    }

    public void webViewBackFailed() {
        if (history.size() != 0) {
            String p = history.get(history.size() - 1);
            history.remove(history.size() - 1);
            switch (p) {
                case "openServer":
                    toggleWebViews(false);
            }
            return;
        }
        // This will usually exit the activity
        Log.i("C", "going super.back");
        super.onBackPressed();
    }

    public void toggleWebViews(boolean showRemote) {
        LinearLayout.LayoutParams local = new LinearLayout.LayoutParams(
                ActionBar.LayoutParams.MATCH_PARENT,
                //showRemote ? 200 : ActionBar.LayoutParams.MATCH_PARENT,
                showRemote ? 0 : ActionBar.LayoutParams.MATCH_PARENT,
                0
        );
        LinearLayout.LayoutParams remote = new LinearLayout.LayoutParams(
                ActionBar.LayoutParams.MATCH_PARENT,
                0,
                showRemote ? 1.0f : 0
        );
        localWebView.setLayoutParams(local);
        remoteWebView.setLayoutParams(remote);
        isRemote = showRemote;
    }

    public void openServer(String url, boolean addToHistory) {
        toggleWebViews(true);
        remoteWebView.loadUrl(url);
        if (addToHistory) {
            history.add("openServer");
        }
    }

}