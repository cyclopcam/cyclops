package org.cyclops;

import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.ProxyConfig;
import androidx.webkit.ProxyController;
import androidx.webkit.WebViewAssetLoader;
import androidx.webkit.WebViewFeature;

import android.annotation.SuppressLint;
import android.app.ActionBar;
import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.net.ConnectivityManager;
import android.net.LinkAddress;
import android.net.LinkProperties;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.Uri;
import android.net.wifi.WifiInfo;
import android.net.wifi.WifiManager;
import android.os.Bundle;
import android.provider.Settings;
import android.util.Log;
import android.view.View;
import android.webkit.CookieManager;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.RelativeLayout;

import java.io.IOException;
import java.net.Proxy;
import java.net.ProxySelector;
import java.net.URI;
import java.util.Collections;
import java.util.List;
import java.net.InetSocketAddress;
import java.net.SocketAddress;

import java.util.ArrayList;
import java.util.concurrent.Executor;
import java.util.concurrent.TimeUnit;

import okhttp3.OkHttpClient;

public class MainActivity extends AppCompatActivity implements Main {
    RelativeLayout rootView;
    View statusBarPlaceholder;
    WebView localWebView; // loads embedded JS code
    WebView remoteWebView; // loads remote JS code from a Cyclops server
    //ImageView darkenOverlay;
    ImageView statusbarScreenGrab;
    Bitmap remoteWebViewScreenGrab;
    LocalContentWebViewClient localClient;
    RemoteWebViewClient remoteClient;
    String dropdownMode = "";
    boolean isRemoteVisible = false;
    boolean isRemoteInFocus = false;
    boolean isOnLAN = false;
    boolean forceProxy = false; // If true, then we never connect via LAN, but always go through proxy
    //Proxy httpProxy = Proxy.NO_PROXY;
    int contentHeight = 0;
    //ConnectivityManager.NetworkCallback networkCallback;
    State.Server currentServer; // The server that our remote webview is pointed at
    String currentNetworkSignature = ""; // Used to detect when we change networks, to avoid sending secrets to a new server with the same IP address
    HttpClient connectivityCheckClient;
    Crypto crypto;

    // Maintain our own history stack, for the tricky transitions between localWebView and remoteWebView.
    // An example of where you need this, is when the user has just scanned the LAN for local servers.
    // Then he clicks on a local server IP. This opens up the remote webview into that server. But then,
    // if he clicks back, then we need to close that remote webview, and return to our local webview.
    ArrayList<String> navigationHistory = new ArrayList<>();

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        crypto = new Crypto();

        connectivityCheckClient = new HttpClient(new OkHttpClient.Builder().callTimeout(300, TimeUnit.MILLISECONDS).build());

        State.global.sharedPref = getSharedPreferences("org.cyclopcam.cyclops.state", Context.MODE_PRIVATE);
        State.global.scanner = new Scanner(this);
        State.global.db = new LocalDB(this);
        State.global.loadAll();

        // Uncomment the following line when testing initial application UI
        //State.global.resetAllState(); // DO NOT COMMIT

        // dev time
        WebView.setWebContentsDebuggingEnabled(true);

        rootView = findViewById(R.id.mainRoot);
        statusBarPlaceholder = findViewById(R.id.statusBarPlaceholder);
        localWebView = findViewById(R.id.localWebView);
        remoteWebView = findViewById(R.id.remoteWebView);
        setupWebView(localWebView);
        setupWebView(remoteWebView);

        //setupHttpProxy();

        // Get rid of white flash when expanding localWebView with screen grab underlay of remoteWebView.
        // OK... even this doesn't work. It's very very hard to get to the bottom of this. Slow it down
        // to debug, and it disappears...
        localWebView.getSettings().setOffscreenPreRaster(true);

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
            showRemoteWebView(false);
        } else {
            State.Server last = State.global.getLastServer();
            if (last == null) {
                last = State.global.servers.get(0);
            }
            switchToServer(last.publicKey);
        }

        setupNetworkMonitor();

        // Our dimensions are not available yet, because layout hasn't happened yet.
        // See https://stackoverflow.com/questions/3591784/views-getwidth-and-getheight-returns-0 for an explanation
        rootView.post(new Runnable() {
            @Override
            public void run() {
                contentHeight = rootView.getHeight() - statusBarPlaceholder.getHeight();
                Log.i("C", "contentHeight = " + contentHeight);
            }
        });

        //openServer("http://192.168.10.15:8080");
    }

    @Override
    protected void onResume() {
        super.onResume();
        // If we've just woken up then it's possible that we've changed networks
        revalidateCurrentConnection();
    }

    @SuppressLint("SetJavaScriptEnabled")
    void setupWebView(WebView webview) {
        WebSettings settings = webview.getSettings();
        // Javascript is not enabled by default!
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        //settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
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
        if (isRemoteInFocus) {
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
                    showRemoteWebView(false);
            }
            return;
        }
        // This will usually exit the activity
        Log.i("C", "going super.back");
        super.onBackPressed();
    }

    // This is called after the user logs in to a server
    public void onLogin(String bearerToken, String sessionCookie) {
        Log.i("C", "onLogin to " + currentServer.publicKey + ", bearerToken: " + bearerToken.substring(0, 4) + "..." + ", sessionCookie: " + sessionCookie.substring(0, 4) + "...");
        State.global.addOrUpdateServer(currentServer.lanIP, currentServer.publicKey, bearerToken, currentServer.name, sessionCookie);
        State.global.setLastServer(currentServer.publicKey);
        localClient.cyRefreshServers(localWebView);
    }

    // Invoked via the Webview when it notices that it can no longer talk to the cyclops server,
    // such as when it loses Wifi connection.
    public void onNetworkDown(String errorMsg) {
        Log.i("C", "onNetworkDown: " + errorMsg);
        revalidateCurrentConnection();
    }

    // This is called after the user logs in to a new server
    //public void notifyRegisteredServersChanged() {
    //    localClient.cyRefreshServers(localWebView);
    //}

    // Show a fullscreen menu that slides in from the left (when user clicks the burger menu on the top-left of the screen)
    public void setLocalWebviewVisibility(String mode) {
        dropdownMode = mode;
        if (dropdownMode.equals("0")) {
            isRemoteInFocus = false;
        } else {
            isRemoteInFocus = true;
        }
        recalculateWebViewLayout();
    }

    public void showRemoteWebView(boolean show) {
        isRemoteVisible = show;
        isRemoteInFocus = true;
        recalculateWebViewLayout();
    }

    // While growing our local content webview, the screen will often flash. This is presumably just
    // because the browser first grows it's size, then does a layout, then repaints, etc, and during that
    // time Android wants to paint, so it paints white. I've tried to preload the browser so that it's ready
    // to render all it's content before the resize, but this doesn't seem to help. So our workaround is this:
    // Before resizing, we set to View.INVISIBLE.
    // After resizing, we set it to View.VISIBLE.
    // This seems to help, but I do still see flashes, particularly the first time that the webview gets enlarged.
    // Now, an unfortunate consequence of hiding the webview while it expands, is that our status bar also
    // get hidden. To work around this, we show a screen grab of the status bar while we make the local web view
    // invisible.
    public void recalculateWebViewLayout() {
        int statusBarHeight = statusBarPlaceholder.getHeight();
        //boolean needOverlay = false;

        RelativeLayout.LayoutParams local = new RelativeLayout.LayoutParams(ActionBar.LayoutParams.MATCH_PARENT, 0);
        if (isRemoteVisible) {
            if (dropdownMode.equals("1")) {
                // The WebView wants to expand. But before expanding it, we make it invisible, so that there
                // is no flicker as it redraws in an intermediate state.
                Log.i("C", "recalculateWebViewLayout dropDown=1, localWebView.height = " + localWebView.getHeight());
                replaceStatusBarWithScreenGrab();
                local.height = ActionBar.LayoutParams.MATCH_PARENT;
                //localWebView.setVisibility(View.INVISIBLE);
                localWebView.setAlpha(0.01f);
            } else if (dropdownMode.equals("2")) {
                // By this stage, the WebView has rendered itself, so we can show it
                Log.i("C", "recalculateWebViewLayout dropDown=2, localWebView.height = " + localWebView.getHeight());
                removeStatusBarScreenGrab();
                local.height = ActionBar.LayoutParams.MATCH_PARENT;
                //localWebView.setVisibility(View.VISIBLE);
                localWebView.setAlpha(1.0f);
            } else {
                // Just status bar at the top
                Log.i("C", "recalculateWebViewLayout dropDown=0, localWebView.height = " + localWebView.getHeight());
                removeStatusBarScreenGrab();
                local.height = statusBarHeight;
            }
        } else {
            Log.i("C", "recalculateWebViewLayout remote hidden");
            local.height = ActionBar.LayoutParams.MATCH_PARENT;
        }

        RelativeLayout.LayoutParams remote = new RelativeLayout.LayoutParams(ActionBar.LayoutParams.MATCH_PARENT, 0);
        if (isRemoteVisible) {
            remote.addRule(RelativeLayout.BELOW, R.id.statusBarPlaceholder);
            remote.height = rootView.getHeight() - statusBarHeight;
        }
        //Log.i("C", rootView.getHeight() + ", " + rootView.getMeasuredHeight() + ", " + statusBarPlaceholder.getHeight() + ", " + statusBarPlaceholder.getMeasuredHeight());

        //if (needOverlay && screenGrab == null) {
        //    saveScreenGrab();
        //} else if (!needOverlay && screenGrab != null) {
        //    screenGrab = null;
        //}

        localWebView.setLayoutParams(local);
        remoteWebView.setLayoutParams(remote);
    }

    // This must be callable from a background thread, which is why it's cached.
    // The reason we need this is so that the local webview can have it's surface 100%
    // ready to go, before we increase it's size to cover the whole screen.
    public int getContentHeight() {
        return contentHeight;
    }

    public Bitmap getRemoteViewScreenGrab() {
        return remoteWebViewScreenGrab;
    }

    public void clearRemoteViewScreenGrab() {
        remoteWebViewScreenGrab = null;
    }

    // this must be idempotent, so that caller can keep calling natcom/getScreenGrab until it returns a bitmap
    public void createRemoteViewScreenGrab() {
        if (remoteWebViewScreenGrab == null) {
            remoteWebViewScreenGrab = getScreenGrabOfView(remoteWebView);
        }
    }

    void replaceStatusBarWithScreenGrab() {
        int statusBarHeight = statusBarPlaceholder.getHeight();
        statusbarScreenGrab = new ImageView(this);
        statusbarScreenGrab.setImageBitmap(getScreenGrabOfView(localWebView));
        RelativeLayout.LayoutParams lp = new RelativeLayout.LayoutParams(0, 0);
        lp.addRule(RelativeLayout.ALIGN_LEFT, R.id.mainRoot);
        lp.addRule(RelativeLayout.ALIGN_TOP, R.id.mainRoot);
        lp.width = ActionBar.LayoutParams.MATCH_PARENT;
        lp.height = statusBarHeight;
        statusbarScreenGrab.setLayoutParams(lp);
        rootView.addView(statusbarScreenGrab);
    }

    void removeStatusBarScreenGrab() {
        rootView.removeView(statusbarScreenGrab);
        statusbarScreenGrab = null;
    }

    Bitmap getScreenGrabOfView(View grabView) {
        if (grabView.getWidth() == 0 || grabView.getHeight() == 0) {
            Log.i("C", "grabView is empty, so just getScreenGrabOfView is returning a 1x1 white bitmap");
            int[] colors = new int[]{0xffffffff};
            Bitmap bmp = Bitmap.createBitmap(1, 1, Bitmap.Config.ARGB_8888);
            return bmp;
        } else {
            Log.i("C", "Grabbing screen size " + grabView.getWidth() + " x " + grabView.getHeight());
            Bitmap cached = grabView.getDrawingCache();
            if (cached != null) {
                Log.i("C", "getDrawingCache() -> size " + cached.getWidth() + " x " + cached.getHeight());
            }
            Bitmap bmp = Bitmap.createBitmap(grabView.getWidth(), grabView.getHeight(), Bitmap.Config.ARGB_8888);
            Canvas c = new Canvas(bmp);
            //rootView.layout()
            grabView.draw(c);
            Log.i("C", "Screen grab done");
            return bmp;
        }
    }

    public String serverLanURL(State.Server server) {
        return Constants.serverLanURL(server.lanIP);
    }

    public String serverLanURL(Scanner.ScannedServer server) {
        return Constants.serverLanURL(server.ip);
    }

    public void revalidateCurrentConnection() {
        if (currentServer != null) {
            switchToServer(currentServer.publicKey);
        }
    }

    // If you call switchToServer and 'server' = 'currentServer', then the function will first check
    // if a change between LAN and proxy is needed. If no change is needed, then it will leave the
    // app as-is.
    // However, if you're doing that, then rather use revalidateCurrentConnection(), to make that
    // explicit.
    public void switchToServer(String publicKey) {
        Context context = this;

        State.Server target = State.global.getServerCopyByPublicKey(publicKey);
        if (target == null) {
            Log.e("C", "Requested to switch to unknown server " + publicKey);
            return;
        }

        // justCheck: If server is not changing, then just check whether connectivity is still OK
        boolean justCheck = currentServer != null && publicKey.equals(currentServer.publicKey);

        State.global.setLastServer(publicKey);

        new Thread(new Runnable() {
            @Override
            public void run() {
                //String proxyOrigin = "https://proxy-cpt.cyclopcam.org";
                String proxyServer = "proxy-cpt.cyclopcam.org";
                String proxyOrigin = "https://" + Crypto.shortKeyForServer(target.publicKey) + ".p.cyclopcam.org";
                Log.i("C", "Set proxy to " + proxyServer + ":8083");
                Log.i("C", "Set target to " + proxyOrigin);
                //try {
                //    httpProxy = new Proxy(Proxy.Type.HTTP, new InetSocketAddress(proxyServer, 8083));
                //} catch (Exception e) {
                //    Log.e("C", "Failed to set proxy: " + e.toString());
                //}

                // Try first to connect over LAN, and if that fails, then fall back to proxy.
                State.Server server = target;
                int storedLanIP = Scanner.parseIP(server.lanIP);
                int wifiIP = Scanner.getWifiIPAddress(context);
                if (storedLanIP != 0 && wifiIP != 0 && Scanner.areIPsInSameSubnet(storedLanIP, wifiIP) && !forceProxy) {
                    String err = Scanner.preflightServerCheck(crypto, connectivityCheckClient, server);
                    if (err == null) {
                        // Refresh state of 'server', because the preflight check can alter it (eg obtain a new session cookie)
                        server = State.global.getServerCopyByPublicKey(server.publicKey);
                        if (justCheck && isOnLAN) {
                            Log.i("C", "Remaining on LAN " + server.lanIP + " for server " + server.publicKey);
                            return;
                        }
                        Log.i("C", "Connecting to LAN " + server.lanIP + " for server " + server.publicKey);
                        isOnLAN = true;
                        //runOnUiThread(() -> navigateToServer(serverLanURL(server), false, server, true));

                        State.Server finalServer = server; // need *another* server instance, because Java
                        runOnUiThread(() -> {
                            String lanURL = serverLanURL(finalServer);
                            CookieManager cookies = CookieManager.getInstance();
                            // SYNC-CYCLOPS-SESSION-COOKIE
                            cookies.setCookie(lanURL, "session=" + finalServer.sessionCookie, (Boolean ok) -> {
                                Log.i("C", "setCookie(LAN) session=" + finalServer.sessionCookie.substring(0, 6) + "... result " + (ok ? "OK" : "Failed"));
                                navigateToServer(lanURL, false, finalServer, true);
                            });
                        });

                        return;
                    } else {
                        Log.i("C", "Preflight check failed for LAN " + server.lanIP + " for server " + server.publicKey + ": " + err);
                    }
                }

                // Fall back to using proxy
                setupHttpProxy("http://" + proxyServer + ":8083");
                if (justCheck && !isOnLAN) {
                    Log.i("C", "Remaining on proxy " + proxyServer + " for server " + server.publicKey);
                    return;
                }
                isOnLAN = false;
                Log.i("C", "Falling back to proxy " + proxyServer + " for server " + server.publicKey);
                State.Server finalServer = server;
                runOnUiThread(() -> {
                    CookieManager cookies = CookieManager.getInstance();
                    // SYNC-CYCLOPS-SERVER-COOKIE
                    cookies.setCookie(proxyOrigin, "CyclopsServerPublicKey=" + finalServer.publicKey, (Boolean ok) -> {
                        Log.i("C", "setCookie(proxy) CyclopsServerPublicKey=" +finalServer.publicKey.substring(0,8) + "... result " + (ok ? "OK" : "Failed"));
                        // SYNC-CYCLOPS-SESSION-COOKIE
                        cookies.setCookie(proxyOrigin, "session=" + finalServer.sessionCookie, (Boolean ok2) -> {
                            String shortCookie = finalServer.sessionCookie.substring(0, 5);
                            Log.i("C", "setCookie(proxy) session=" + shortCookie + "... result " + (ok2 ? "OK" : "Failed"));
                            navigateToServer(proxyOrigin, false, finalServer, true);
                        });

                    });
                });
            }
        }).start();
    }

    public void navigateToScannedLocalServer(String publicKey) {
        // We reference the ScannedServer object here so that details such as
        // the hostname and LAN IP can filter through in case the user logs into this server.
        Scanner.ScannedServer s = State.global.scanner.getScannedServer(publicKey);
        if (s == null) {
            Log.i("C", "navigateToScannedLocalServer failed to find server " + publicKey);
            return;
        }
        String baseUrl = serverLanURL(s);

        // Clone ScannedServer into a State.Server object, which carries all of the same
        // relevant details. We'll use this later, if the user logs in (see currentServer in onLogin)
        State.Server tmp = new State.Server();
        tmp.lanIP = s.ip;
        tmp.name = s.hostname;
        tmp.publicKey = s.publicKey;

        // Clear the session cookie for the host, so that we don't somehow get tricked into revealing
        // a session key for an existing server that we talk to, which happens to have the same IP
        // as a freshly scanned server.
        CookieManager cookies = CookieManager.getInstance();
        // SYNC-CYCLOPS-SESSION-COOKIE
        cookies.setCookie(baseUrl, "session=x", (Boolean ok) -> {
            Log.i("C", "setCookie session=x result " + (ok ? "OK" : "Failed"));
            navigateToServer(baseUrl, true, tmp, false);
        });
    }

    // This is private to remind you that you must ensure the cookie state is good before navigating to a server
    private void navigateToServer(String url, boolean addToNavigationHistory, State.Server server, boolean preserveURL) {
        currentServer = server.copy();
        showRemoteWebView(true);
        String currentURL = remoteWebView.getUrl();
        if (preserveURL && currentURL != null) {
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
        remoteWebView.loadUrl(url);
        if (addToNavigationHistory) {
            navigationHistory.add("openServer");
        }
    }

    // This doesn't apply to WebKit, so it's basically useless here.
    // It applies to java.net stuff, so I'm leaving it here as a reminder.
    /*
    private void setupHttpProxy() {
        ProxySelector.setDefault(new ProxySelector() {
            @Override
            public List<Proxy> select(URI uri) {
                Log.e("C", "FOOBAR");
                if (uri.getHost().contains(".cyclopcam.org")) {
                    Log.i("C", "Using proxy for " + uri.getHost());
                    return Collections.singletonList(httpProxy);
                } else {
                    Log.i("C", "NOT using proxy for " + uri.getHost());
                    return Collections.singletonList(Proxy.NO_PROXY);
                }
            }

            @Override
            public void connectFailed(URI uri, SocketAddress sa, IOException ioe) {
                ioe.printStackTrace();
            }
        });
    }
    */

    // Attempt #2
    // This works!
    public void setupHttpProxy(String proxyUrl) {
        ProxyController proxyController;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.PROXY_OVERRIDE)) {
            Log.e("C", "Proxy override is not supported");
            return;
        } else {
            proxyController = ProxyController.getInstance();
        }

        ProxyConfig proxyConfig = new ProxyConfig.Builder()
                .addProxyRule(proxyUrl, ProxyConfig.MATCH_HTTPS)
                .bypassSimpleHostnames()
                .build();

        Executor executor = new Executor() {
            @Override
            public void execute(Runnable command) {
                command.run();
            }
        };

        Runnable listener = new Runnable() {
            @Override
            public void run() {
                Log.i("C", "Proxy override set to " + proxyUrl);
            }
        };

        proxyController.setProxyOverride(proxyConfig, executor, listener);
    }

    public void setupNetworkMonitor() {
        ConnectivityManager connectivityManager = getSystemService(ConnectivityManager.class);

        // NOTE: There seems to be a notable case where this doesn't work, which is if our activity is
        // put to sleep, and then wakes up on another network. I guess we should do a ping when our
        // activity resumes or something.

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
                Log.i("C", "The default network changed link properties: " + linkProperties);

                String networkSignature = linkProperties.getInterfaceName() + " ";
                for (LinkAddress addr : linkProperties.getLinkAddresses()) {
                    networkSignature += "," + addr.toString();
                }

                // In order to detect the SSID and BSSID, we need location data enabled.
                // See https://stackoverflow.com/questions/21391395/get-ssid-when-wifi-is-connected
                // This is why we have ACCESS_COARSE_LOCATION and ACCESS_FINE_LOCATION permissions in our manifest.
                // meh... there are just so many answers on that page, and it's not working for me, so I'm just
                // going to disable it. The only really robust solution is to run a wireguard HTTP proxy right on
                // the phone...
                WifiManager wifi = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
                if (wifi != null) {
                    WifiInfo info = wifi.getConnectionInfo();
                    if (info != null) {
                        //String ssid = info.getSSID();
                        networkSignature += " wifi:" + info.getSSID() + info.getBSSID();
                    }
                }
                Log.i("C", "Old network signature: " + currentNetworkSignature);
                Log.i("C", "New network signature: " + networkSignature);

                if (!networkSignature.equals(currentNetworkSignature)) {
                    currentNetworkSignature = networkSignature;
                    revalidateCurrentConnection();
                }
            }
        });
    }


}