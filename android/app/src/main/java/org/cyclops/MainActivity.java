package org.cyclops;

import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.WebViewAssetLoader;

import android.annotation.SuppressLint;
import android.app.ActionBar;
import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.Uri;
import android.os.Bundle;
import android.util.Log;
import android.view.View;
import android.webkit.CookieManager;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.RelativeLayout;

import java.util.ArrayList;
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
    int contentHeight = 0;
    //ConnectivityManager.NetworkCallback networkCallback;
    State.Server currentServer;
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
        State.global.db = new LocalDB(this);
        State.global.loadAll();

        // dev time
        WebView.setWebContentsDebuggingEnabled(true);

        rootView = findViewById(R.id.mainRoot);
        statusBarPlaceholder = findViewById(R.id.statusBarPlaceholder);
        localWebView = findViewById(R.id.localWebView);
        remoteWebView = findViewById(R.id.remoteWebView);
        setupWebView(localWebView);
        setupWebView(remoteWebView);

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
            State.Server current = State.global.getCurrentServer();
            if (current == null) {
                current = State.global.servers.get(0);
            }
            switchToServer(current);
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

    // Show a fullscreen menu that slides in from the left (when user clicks the burger menu on the top-left of the screen)
    public void showMenu(String mode) {
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
        Log.i("C", "Grabbing screen size " + grabView.getWidth() + " x " + grabView.getHeight());
        Bitmap bmp = Bitmap.createBitmap(grabView.getWidth(), grabView.getHeight(), Bitmap.Config.ARGB_8888);
        Canvas c = new Canvas(bmp);
        //rootView.layout()
        grabView.draw(c);
        return bmp;
    }

    public void switchToServerByPublicKey(String publicKey) {
        State.Server server = State.global.getServerCopyByPublicKey(publicKey);
        if (server == null) {
            Log.e("C", "Requested to switch to unknown server " + publicKey);
            return;
        }
        State.global.setCurrentServer(publicKey);
        switchToServer(server);
    }

    // If you call switchToServer and 'server' = 'currentServer', then the function will first check
    // if a change between LAN and proxy is needed. If no change is needed, then it will leave the
    // app as-is.
    public void switchToServer(State.Server server) {
        Context context = this;

        // justCheck: If server is not changing, then just check whether connectivity is still OK
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
                    // TODO: incorporate cryptographic challenge to test if server is who he claims to be
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
        showRemoteWebView(true);
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
                Log.e("C", "The default network changed link properties: " + linkProperties);
                if (!linkProperties.getInterfaceName().equals(currentNetworkInterfaceName)) {
                    currentNetworkInterfaceName = linkProperties.getInterfaceName();
                    if (currentServer != null) {
                        switchToServer(currentServer);
                    }
                }
            }
        });
    }


}