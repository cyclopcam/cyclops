package org.cyclops;

import androidx.annotation.NonNull;
import androidx.annotation.RequiresApi;
import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.WebViewAssetLoader;

import android.annotation.SuppressLint;
import android.app.ActionBar;
import android.content.Context;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.NetworkRequest;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.util.Base64;
import android.util.Log;
import android.webkit.CookieManager;
import android.webkit.ValueCallback;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.LinearLayout;

import java.util.ArrayList;
import java.util.concurrent.TimeUnit;

import okhttp3.Cookie;
import okhttp3.OkHttpClient;

public class MainActivity extends AppCompatActivity implements Main {
    WebView localWebView; // loads embedded JS code
    WebView remoteWebView; // loads remote JS code from a Cyclops server
    LocalContentWebViewClient localClient;
    RemoteWebViewClient remoteClient;
    boolean isRemote = false;
    //ConnectivityManager.NetworkCallback networkCallback;
    State.Server currentServer;
    boolean isOnLAN = false;
    String currentNetworkInterfaceName = "";

    // Maintain our own history stack, for the tricky transitions between localWebView and remoteWebView.
    // An example of where you need this, is when the user has just scanned the LAN for local servers.
    // Then he clicks on a local server IP. This opens up the remote webview into that server. But then,
    // if he clicks back, then we need to close that remote webview, and return to our local webview.
    ArrayList<String> navigationHistory = new ArrayList<>();

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        State.global.sharedPref = getSharedPreferences("org.cyclopcam.cyclops.state", Context.MODE_PRIVATE);
        State.global.scanner = new Scanner(this);
        State.global.connector = new Connector(this);
        State.global.db = new LocalDB(this);
        State.global.loadAll();

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
        localClient = new LocalContentWebViewClient(assetLoader, this, this);
        localWebView.setWebViewClient(localClient);
        localWebView.loadUrl("https://appassets.androidplatform.net/assets/index.html");

        remoteClient = new RemoteWebViewClient(this, this);
        remoteWebView.setWebViewClient(remoteClient);

        if (State.global.servers.size() == 0) {
            toggleWebViews(false);
        } else {
            State.Server current = State.global.getCurrentServer();
            if (current == null) {
                current = State.global.servers.get(0);
            }
            switchToServer(current);
        }

        setupNetworkMonitor();

        //openServer("http://192.168.10.15:8080");
    }

    @SuppressLint("SetJavaScriptEnabled")
    void setupWebView(WebView webview) {
        WebSettings settings = webview.getSettings();
        // Javascript is not enabled by default!
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setMediaPlaybackRequiresUserGesture(false);

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
        if (navigationHistory.size() != 0) {
            String p = navigationHistory.get(navigationHistory.size() - 1);
            navigationHistory.remove(navigationHistory.size() - 1);
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

    // If you call switchToServer and 'server' = 'currentServer', then the function will first check
    // if a change between LAN and proxy is needed. If no change is needed, then it will leave the
    // app as-is.
    public void switchToServer(State.Server server) {
        Context context = this;

        // Just check if connectivity is still fine. If fine, then don't reload.
        boolean justCheck = currentServer != null && server.publicKey.equals(currentServer.publicKey);
        currentServer = server.copy();

        new Thread(new Runnable() {
            @Override
            public void run() {
                // Try first to connect over LAN, and if that fails, then fall back to proxy.
                int storedLanIP = Scanner.parseIP(server.lanIP);
                int wifiIP = Scanner.getWifiIPAddress(context);
                if (storedLanIP != 0 && wifiIP != 0 && Scanner.areIPsInSameSubnet(storedLanIP, wifiIP)) {
                    OkHttpClient client = new OkHttpClient.Builder().callTimeout(300, TimeUnit.MILLISECONDS).build();
                    JSAPI.PingResponseJSON ping = Scanner.isCyclopsServer(client, server.lanIP);
                    if (ping != null) {
                        if (justCheck && isOnLAN) {
                            Log.i("C", "Remaining on LAN " + server.lanIP + " for server " + server.publicKey);
                            return;
                        }
                        Log.i("C", "Connecting to LAN " + server.lanIP + " for server " + server.publicKey);
                        isOnLAN = true;
                        runOnUiThread(() -> navigateToServer("http://" + server.lanIP + ":" + Constants.ServerPort, false, server));
                        return;
                    }
                }

                // Fall back to using proxy
                String proxyOrigin = "https://proxy-cpt.cyclopcam.org";
                if (justCheck && !isOnLAN) {
                    Log.i("C", "Remaining on proxy " + proxyOrigin + " for server " + server.publicKey);
                    return;
                }
                isOnLAN = false;
                Log.i("C", "Falling back to proxy " + proxyOrigin + " for server " + server.publicKey);
                runOnUiThread(() -> {
                    CookieManager cookies = CookieManager.getInstance();
                    // SYNC-CYCLOPS-SERVER-COOKIE
                    cookies.setCookie(proxyOrigin, "CyclopsServerPublicKey=" + server.publicKey, (Boolean ok) -> {
                        Log.i("C", "setCookie result " + (ok ? "OK" : "Failed"));
                        navigateToServer(proxyOrigin, false, server);
                    });
                });
            }
        }).start();
    }

    public void navigateToServer(String url, boolean addToNavigationHistory, State.Server server) {
        toggleWebViews(true);
        String currentURL = remoteWebView.getUrl();
        if (currentURL != null) {
            // Preserve the Vue route when switching between LAN and proxy.
            // Unfortunately this doesn't preserve our entire UI state. THAT is left as an exercise for the reader (i.e. not trivial).
            Uri current = Uri.parse(currentURL);
            Uri next = Uri.parse(url);
            if (current.getPath().length() > 0 && (next.getPath().equals("") || next.getPath().equals("/"))) {
                if (url.endsWith("/")) {
                    url = url.substring(0, url.length() - 1);
                }
                url += current.getPath();
            }
        }
        Log.i("C", "navigateToServer. currentURL = " + currentURL + ". New URL = " + url);
        remoteClient.setServer(server);
        remoteClient.setUrl(Uri.parse(url));
        remoteWebView.loadUrl(url);
        if (addToNavigationHistory) {
            navigationHistory.add("openServer");
        }
    }

    public void setupNetworkMonitor() {
        ConnectivityManager connectivityManager = getSystemService(ConnectivityManager.class);

        connectivityManager.registerDefaultNetworkCallback(new ConnectivityManager.NetworkCallback() {
            @Override
            public void onAvailable(Network network) {
                //Log.e("C", "The default network is now: " + network);
            }

            @Override
            public void onLost(Network network) {
                //Log.e("C", "The application no longer has a default network. The last default network was " + network);
            }

            @Override
            public void onCapabilitiesChanged(Network network, NetworkCapabilities networkCapabilities) {
                //Log.e("C", "The default network changed capabilities: " + networkCapabilities);
            }

            @Override
            public void onLinkPropertiesChanged(Network network, LinkProperties linkProperties) {
                Log.e("C", "The default network changed link properties: " + linkProperties);
                if (!linkProperties.getInterfaceName().equals(currentNetworkInterfaceName)) {
                    currentNetworkInterfaceName = linkProperties.getInterfaceName();
                    if (currentServer != null) {
                        switchToServer(currentServer);
                    }
                }
            }
        });

        /*
        NetworkRequest networkRequest = new NetworkRequest.Builder()
                .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                .addTransportType(NetworkCapabilities.TRANSPORT_WIFI)
                .addTransportType(NetworkCapabilities.TRANSPORT_CELLULAR)
                .build();

        ConnectivityManager.NetworkCallback networkCallback = new ConnectivityManager.NetworkCallback() {
            @Override
            public void onAvailable(@NonNull Network network) {
                Log.i("C", "networkCallback.onAvailable");
                super.onAvailable(network);
                if (currentServer != null) {
                    switchToServer(currentServer);
                }
            }

            @Override
            public void onLost(@NonNull Network network) {
                Log.i("C", "networkCallback.onLost");
                super.onLost(network);
            }

            @Override
            public void onCapabilitiesChanged(@NonNull Network network, @NonNull NetworkCapabilities networkCapabilities) {
                Log.i("C", "networkCallback.onCapabilitiesChanged");
                super.onCapabilitiesChanged(network, networkCapabilities);
                final boolean unmetered = networkCapabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED);
            }
        };

        ConnectivityManager connectivityManager = (ConnectivityManager) getSystemService(ConnectivityManager.class);
        connectivityManager.requestNetwork(networkRequest, networkCallback);
         */
    }


}