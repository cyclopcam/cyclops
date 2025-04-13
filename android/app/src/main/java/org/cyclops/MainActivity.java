package org.cyclops;

//import static org.cyclops.Accounts.RC_SIGN_IN;

import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.ProxyConfig;
import androidx.webkit.ProxyController;
import androidx.webkit.WebViewAssetLoader;
import androidx.webkit.WebViewFeature;

import android.annotation.SuppressLint;
import android.app.ActionBar;
import android.content.Context;
import android.content.Intent;
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
import androidx.annotation.Nullable;

//import com.google.android.gms.auth.api.signin.GoogleSignIn;
//import com.google.android.gms.auth.api.signin.GoogleSignInAccount;
//import com.google.android.gms.common.api.ApiException;
//import com.google.android.gms.tasks.Task;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.concurrent.Executor;
import java.util.concurrent.TimeUnit;

import okhttp3.OkHttpClient;

public class MainActivity extends AppCompatActivity implements Main {
    private static final String TAG = "cyclops";

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
    boolean mIsLoggingIn = false; // True if we're busy trying to login to a server
    String currentNetworkSignature = ""; // Used to detect when we change networks, to avoid sending secrets to a new server with the same IP address
    HttpClient connectivityCheckClient;
    Crypto crypto;
    Accounts accounts;

    // Maintain our own history stack, for the tricky transitions between localWebView and remoteWebView.
    // An example of where you need this, is when the user has just scanned the LAN for local servers.
    // Then he clicks on a local server IP. This opens up the remote webview into that server. But then,
    // if he clicks back, then we need to close that remote webview, and return to our local webview.
    ArrayList<String> navigationHistory = new ArrayList<>();

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        Log.i(TAG, "MainActivity onCreate");
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

        accounts = new Accounts();

        // dev time
        //WebView.setWebContentsDebuggingEnabled(true);

        rootView = findViewById(R.id.mainRoot);
        statusBarPlaceholder = findViewById(R.id.statusBarPlaceholder);
        localWebView = findViewById(R.id.localWebView);
        remoteWebView = findViewById(R.id.remoteWebView);
        setupWebView(localWebView);
        setupWebView(remoteWebView);

        //setupHttpProxy();

        remoteClient = new RemoteWebViewClient(this, this);
        remoteWebView.setWebViewClient(remoteClient);

        accounts.debugPrintSigningCert(this);

        // Clear login token with accounts.cyclopcam.org (for testing fresh OAuth login)
        //State.global.setAccountsToken("");

        String accountsToken = State.global.getAccountsToken();
        Log.i(TAG, "accountsToken: " + accountsToken);

        // Handle cyclops://auth redirect if app was launched via URI
        boolean haveRedirect = handleRedirect(getIntent());
        // Force sign-in (used when testing)
        //if (State.global.getAccountsToken().equals("")) {
        //    accounts.signinWeb(this, "");
        //}

        // Delay the startup of our local appui webview, so that our "isLoggingIn" state is correct
        // before appui's login runs. appui will check if we have zero servers, and are not logging in,
        // and will bring itself to the forefront, and show the welcome/scan for servers screen.
        // We don't want that to happy if we're busy logging in. This was first a problem when adding
        // OAuth via custom chrome tabs, because our activity gets shutdown and resumed.
        // handleRedirect() is where isLoggingIn gets set, in the above case.
        startLocalAppUIWebView();

        if (!haveRedirect) {
            if (State.global.servers.size() == 0) {
                showRemoteWebView("onCreate servers.size() == 0", false);
            } else {
                State.Server last = State.global.getLastServer();
                if (last == null) {
                    last = State.global.servers.get(0);
                }
                switchToServer(last.publicKey);
            }
            // This can interfere with the login process, so disable it while performing initial auth/login
            setupNetworkMonitor();
        }

        // Our dimensions are not available yet, because layout hasn't happened yet.
        // See https://stackoverflow.com/questions/3591784/views-getwidth-and-getheight-returns-0 for an explanation
        rootView.post(new Runnable() {
            @Override
            public void run() {
                contentHeight = rootView.getHeight() - statusBarPlaceholder.getHeight();
                Log.i(TAG, "contentHeight = " + contentHeight);
            }
        });

        //openServer("http://192.168.10.15:8080");
    }

    @Override
    protected void onNewIntent(Intent intent) {
        Log.i(TAG, "MainActivity onNewIntent");
        super.onNewIntent(intent);
        // Handle cyclops://auth redirect if app was already running
        handleRedirect(intent);
    }

    @Override
    protected void onResume() {
        Log.i(TAG, "MainActivity onResume");
        super.onResume();
        // If we've just woken up then it's possible that we've changed networks
        revalidateCurrentConnection();
    }

    @Override
    public boolean isLoggingIn() {
        return mIsLoggingIn;
    }

    void startLocalAppUIWebView() {
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
                    showRemoteWebView("webViewBackFailed", false);
            }
            return;
        }
        // This will usually exit the activity
        Log.i(TAG, "going super.back");
        super.onBackPressed();
    }

    // This is called after the user logs in to a server
    public void onLogin(String bearerToken, String sessionCookie) {
        Log.i(TAG, "onLogin to " + currentServer.publicKey +
                ", bearerToken: " + bearerToken.substring(0, 4) +
                "..., sessionCookie: " + sessionCookie.substring(0, 4));
        mIsLoggingIn = false;
        State.global.addOrUpdateServer(currentServer.lanIP, currentServer.publicKey, bearerToken, currentServer.name, sessionCookie);
        State.global.setLastServer(currentServer.publicKey);
        localClient.cyRefreshServers(localWebView);
        switchToServer(currentServer.publicKey);
    }

    // Invoked via the Webview when it notices that it can no longer talk to the cyclops server,
    // such as when it loses Wifi connection.
    public void onNetworkDown(String errorMsg) {
        Log.i(TAG, "onNetworkDown: " + errorMsg);
        revalidateCurrentConnection();
    }

    // Invoked by the Remote webview when the user wants to create the initial user via OAuth.
    // Basically what he's asking for is an IdentityToken to accounts.cyclopcam.org, and for us to
    // ensure that the identity there has been associated with a Microsoft account.
    public void requestOAuthLogin(String purpose, String provider) {
        Log.i(TAG, "requestOAuthLogin");
        remoteClient.cySetProgressMessage(remoteWebView, "Searching for tokens...");
        boolean isSignedIn = false;
        try {
            isSignedIn = accounts.isSignedinWithOAuthProvider(provider, State.global.getAccountsToken());
        } catch (RuntimeException e) {
            // Likely network error.
            remoteClient.cySetProgressMessage(remoteWebView, "ERROR:" + e.toString());
            return;
        }

        if (isSignedIn) {
            // Acquire an IdentityToken and send it to the WebView
            remoteClient.cySetProgressMessage(remoteWebView, "Almost there...");
            try {
                Accounts.CreateTokenJSON identityToken = accounts.getIdentityToken(State.global.getAccountsToken());
                remoteClient.cySetIdentityToken(remoteWebView, identityToken.token);
                //remoteClient.cySetProgressMessage(remoteWebView, "Use this: " + identityToken.token);
            } catch(RuntimeException e) {
                remoteClient.cySetProgressMessage(remoteWebView, "ERROR:" + e.getMessage());
            }
        } else {
            // First sign in via OAuth, then acquire an IdentityToken and send it to the WebView
            remoteClient.cySetProgressMessage(remoteWebView, "Redirecting to OAuth signin...");

            // Save our current state, so that we can restore it after the user has logged in
            State.SavedActivity save = new State.SavedActivity();
            save.activity = State.SAVEDACTIVITY_NEWSERVER_LOGIN;
            save.scannedServer = State.global.scanner.getScannedServer(currentServer.publicKey);
            save.oauthProvider = provider;
            State.global.saveActivity(save);

            accounts.signinWeb(this, provider);
        }
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

    public void showRemoteWebView(String reason, boolean show) {
        Log.i(TAG, "showRemoteWebView: " + reason + ", " + show);
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
                Log.i(TAG, "recalculateWebViewLayout dropDown=1, localWebView.height = " + localWebView.getHeight());
                replaceStatusBarWithScreenGrab();
                local.height = ActionBar.LayoutParams.MATCH_PARENT;
                //localWebView.setVisibility(View.INVISIBLE);
                localWebView.setAlpha(0.01f);
            } else if (dropdownMode.equals("2")) {
                // By this stage, the WebView has rendered itself, so we can show it
                Log.i(TAG, "recalculateWebViewLayout dropDown=2, localWebView.height = " + localWebView.getHeight());
                removeStatusBarScreenGrab();
                local.height = ActionBar.LayoutParams.MATCH_PARENT;
                //localWebView.setVisibility(View.VISIBLE);
                localWebView.setAlpha(1.0f);
            } else {
                // Just status bar at the top
                Log.i(TAG, "recalculateWebViewLayout dropDown=0, localWebView.height = " + localWebView.getHeight());
                removeStatusBarScreenGrab();
                local.height = statusBarHeight;
            }
        } else {
            Log.i(TAG, "recalculateWebViewLayout remote hidden");
            local.height = ActionBar.LayoutParams.MATCH_PARENT;
        }

        RelativeLayout.LayoutParams remote = new RelativeLayout.LayoutParams(ActionBar.LayoutParams.MATCH_PARENT, 0);
        if (isRemoteVisible) {
            remote.addRule(RelativeLayout.BELOW, R.id.statusBarPlaceholder);
            remote.height = rootView.getHeight() - statusBarHeight;
        }
        //Log.i(TAG, rootView.getHeight() + ", " + rootView.getMeasuredHeight() + ", " + statusBarPlaceholder.getHeight() + ", " + statusBarPlaceholder.getMeasuredHeight());

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
            Log.i(TAG, "grabView is empty, so just getScreenGrabOfView is returning a 1x1 white bitmap");
            int[] colors = new int[]{0xffffffff};
            Bitmap bmp = Bitmap.createBitmap(1, 1, Bitmap.Config.ARGB_8888);
            return bmp;
        } else {
            Log.i(TAG, "Grabbing screen size " + grabView.getWidth() + " x " + grabView.getHeight());
            Bitmap cached = grabView.getDrawingCache();
            if (cached != null) {
                Log.i(TAG, "getDrawingCache() -> size " + cached.getWidth() + " x " + cached.getHeight());
            }
            Bitmap bmp = Bitmap.createBitmap(grabView.getWidth(), grabView.getHeight(), Bitmap.Config.ARGB_8888);
            Canvas c = new Canvas(bmp);
            //rootView.layout()
            grabView.draw(c);
            Log.i(TAG, "Screen grab done");
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

    public void serverDeleted(String publicKey) {
        // If the server that we were connected to has been deleted, then connect to something else.
        if (currentServer != null && currentServer.publicKey.equals(publicKey)) {
            State.Server any = State.global.getAnyServer();
            if (any != null) {
                switchToServer(any.publicKey);
            }
        }
    }

    // If you call switchToServer and publicKey = '<public key of current server>', then the function will first check
    // if a change between LAN and proxy is needed. If no change is needed, then it will leave the
    // app as-is.
    // However, if you're doing that, then rather use revalidateCurrentConnection(), to make that
    // explicit.
    public void switchToServer(String publicKey) {
        Context context = this;

        State.Server target = State.global.getServerCopyByPublicKey(publicKey);
        if (target == null) {
            Log.e(TAG, "Requested to switch to unknown server " + publicKey);
            return;
        }

        // justCheck: If server is not changing, then just check whether connectivity is still OK
        boolean justCheck = currentServer != null && publicKey.equals(currentServer.publicKey);

        if (!justCheck) {
            State.global.setLastServer(publicKey);
        }

        // If we're switching servers, don't preserve the remote URL
        final boolean preserveRemoteUrl = justCheck;

        new Thread(new Runnable() {
            @Override
            public void run() {
                String proxyServer = "proxy-cpt.cyclopcam.org";
                String proxyOrigin = "https://" + Crypto.shortKeyForServer(target.publicKey) + ".p.cyclopcam.org";
                Log.i(TAG, "proxyServer: " + proxyServer + ":8083");
                Log.i(TAG, "proxyOrigin: " + proxyOrigin);
                //try {
                //    httpProxy = new Proxy(Proxy.Type.HTTP, new InetSocketAddress(proxyServer, 8083));
                //} catch (Exception e) {
                //    Log.e(TAG, "Failed to set proxy: " + e.toString());
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
                            Log.i(TAG, "Remaining on LAN " + server.lanIP + " for server " + server.publicKey);
                            return;
                        }
                        Log.i(TAG, "Connecting to LAN " + server.lanIP + " for server " + server.publicKey);
                        isOnLAN = true;

                        State.Server finalServer = server; // need *another* server instance, because Java
                        runOnUiThread(() -> {
                            String lanURL = serverLanURL(finalServer);
                            CookieManager cookies = CookieManager.getInstance();
                            // SYNC-CYCLOPS-SESSION-COOKIE
                            cookies.setCookie(lanURL, "session=" + finalServer.sessionCookie, (Boolean ok) -> {
                                Log.i(TAG, "setCookie(LAN) session=" + finalServer.sessionCookie.substring(0, 6) + "... result " + (ok ? "OK" : "Failed"));
                                navigateToServer("Reconnecting on LAN", lanURL, false, finalServer, preserveRemoteUrl, null, false);
                            });
                        });

                        return;
                    } else {
                        Log.i(TAG, "Preflight check failed for LAN " + server.lanIP + " for server " + server.publicKey + ": " + err);
                    }
                }

                // Fall back to using proxy
                setupHttpProxy("http://" + proxyServer + ":8083");
                if (justCheck && !isOnLAN) {
                    Log.i(TAG, "Remaining on proxy " + proxyServer + " for server " + server.publicKey);
                    return;
                }
                isOnLAN = false;
                Log.i(TAG, "Falling back to proxy " + proxyServer + " for server " + server.publicKey);
                State.Server finalServer = server;
                runOnUiThread(() -> {
                    CookieManager cookies = CookieManager.getInstance();
                    // SYNC-CYCLOPS-SERVER-COOKIE
                    cookies.setCookie(proxyOrigin, "CyclopsServerPublicKey=" + finalServer.publicKey, (Boolean ok) -> {
                        Log.i(TAG, "setCookie(proxy) CyclopsServerPublicKey=" +finalServer.publicKey.substring(0,8) + "... result " + (ok ? "OK" : "Failed"));
                        // SYNC-CYCLOPS-SESSION-COOKIE
                        cookies.setCookie(proxyOrigin, "session=" + finalServer.sessionCookie, (Boolean ok2) -> {
                            String shortCookie = finalServer.sessionCookie.substring(0, 5);
                            Log.i(TAG, "setCookie(proxy) session=" + shortCookie + "... result " + (ok2 ? "OK" : "Failed"));
                            navigateToServer("Reconnecting via proxy", proxyOrigin, false, finalServer, preserveRemoteUrl, null, false);
                        });

                    });
                });
            }
        }).start();
    }

    public void navigateToScannedLocalServer(String publicKey, String path, HashMap<String,String> queryParams) {
        // We reference the ScannedServer object here so that details such as
        // the hostname and LAN IP can filter through in case the user logs into this server.
        Scanner.ScannedServer s = State.global.scanner.getScannedServer(publicKey);
        if (s == null) {
            Log.i(TAG, "navigateToScannedLocalServer failed to find server " + publicKey);
            return;
        }
        String baseUrl = serverLanURL(s);
        if (path != null && !path.equals("")) {
            baseUrl += path;
        }

        // Clone ScannedServer into a State.Server object, which carries all of the same
        // relevant details. We'll use this later, if the user logs in (see currentServer in onLogin)
        State.Server tmp = new State.Server();
        tmp.lanIP = s.ip;
        tmp.name = s.hostname;
        tmp.publicKey = s.publicKey;

        final String baseUrlCopy = baseUrl;

        // Clear the session cookie for the host, so that we don't somehow get tricked into revealing
        // a session key for an existing server that we talk to, which happens to have the same IP
        // as a freshly scanned server.
        CookieManager cookies = CookieManager.getInstance();
        // SYNC-CYCLOPS-SESSION-COOKIE
        cookies.setCookie(baseUrl, "session=x", (Boolean ok) -> {
            Log.i(TAG, "setCookie session=x result " + (ok ? "OK" : "Failed"));
            navigateToServer("Scanned local", baseUrlCopy, true, tmp, false, queryParams, true);
        });
    }

    // This is private to remind you that you must ensure the cookie state is good before navigating to a server
    private void navigateToServer(String reason, String url, boolean addToNavigationHistory, State.Server server, boolean preserveURL, HashMap<String,String> queryParams, boolean forLogin) {
        currentServer = server.copy();
        mIsLoggingIn = forLogin;
        showRemoteWebView("navigateToServer (" + reason + ")", true);
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
        if (queryParams != null) {
            Uri.Builder builder = Uri.parse(url).buildUpon();
            for (String key : queryParams.keySet()) {
                builder.appendQueryParameter(key, queryParams.get(key));
            }
            url = builder.build().toString();
        }
        Log.i(TAG, "navigateToServer. reason = " + reason + ". currentURL = " + currentURL + ". New URL = " + url);
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
                Log.e(TAG, "FOOBAR");
                if (uri.getHost().contains(".cyclopcam.org")) {
                    Log.i(TAG, "Using proxy for " + uri.getHost());
                    return Collections.singletonList(httpProxy);
                } else {
                    Log.i(TAG, "NOT using proxy for " + uri.getHost());
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
            Log.e(TAG, "Proxy override is not supported");
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
                Log.i(TAG, "Proxy override set to " + proxyUrl);
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
                //Log.e(TAG, "The default network is now: " + network);
            }

            @Override
            public void onLost(Network network) {
                //Log.e(TAG, "The application no longer has a default network. The last default network was " + network);
            }

            @Override
            public void onCapabilitiesChanged(Network network, NetworkCapabilities networkCapabilities) {
                //Log.e(TAG, "The default network changed capabilities: " + networkCapabilities);
            }

            @Override
            public void onLinkPropertiesChanged(Network network, LinkProperties linkProperties) {
                Log.i(TAG, "The default network changed link properties: " + linkProperties);

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
                Log.i(TAG, "Old network signature: " + currentNetworkSignature);
                Log.i(TAG, "New network signature: " + networkSignature);

                if (!networkSignature.equals(currentNetworkSignature)) {
                    currentNetworkSignature = networkSignature;
                    revalidateCurrentConnection();
                }
            }
        });
    }

    // For native oauth (not used)
    /*
    @Override
    protected void onActivityResult(int requestCode, int resultCode, @Nullable Intent data) {
        super.onActivityResult(requestCode, resultCode, data);

        if (requestCode == RC_SIGN_IN) {
            Task<GoogleSignInAccount> task = GoogleSignIn.getSignedInAccountFromIntent(data);
            try {
                GoogleSignInAccount account = task.getResult(ApiException.class);
                String idToken = account.getIdToken();
                Log.d(TAG, "ID Token: " + idToken);
                //sendIdTokenToBackend(idToken);
            } catch (ApiException e) {
                Log.w(TAG, "Sign-in failed with code: " + e.getStatusCode() + ", message: " + e.getMessage(), e);
            }
        }
    }
    */

    // For browser based oauth
    // You can simulate this using adb:
    // adb shell am start -a android.intent.action.VIEW -d "cyclops://auth?token=secret"
    // This is the final phase of logging into accounts.cyclopcam.org via OAuth, via a Chrome Custom Tab)
    // Our shared-secret token comes in via a URL query parameter called "token"
    private boolean handleRedirect(Intent intent) {
        if (intent == null || intent.getData() == null) {
            return false; // No redirect to handle
        }

        Uri redirectUri = intent.getData();
        Log.d(TAG, "handleRedirect: " + redirectUri.toString());
        // V1 uses custom scheme deep links. We're using V1
        boolean isV1Url = "cyclops".equals(redirectUri.getScheme()) && "auth".equals(redirectUri.getHost());
        // V2 uses https deep links. Not using this, but it does work (via cyclopcam.org/.well-known/assetlinks.json).
        boolean isV2Url = "https".equals(redirectUri.getScheme()) && "cyclopcam.org".equals(redirectUri.getHost()) && redirectUri.getPath().startsWith("/android-auth");
        if (isV1Url || isV2Url) {
            // Extract session token from query parameter
            String sessionToken = redirectUri.getQueryParameter("token");
            if (sessionToken != null) {
                Log.d(TAG, "Received session token: " + sessionToken);
                State.global.setAccountsToken(sessionToken);
                mIsLoggingIn = true;
                resumeSavedActivity();
            } else {
                Log.e(TAG, "No session token in redirect URI: " + redirectUri.toString());
            }
            return true;
        }
        return false;
    }

    // Load our previous app state out of the shared preferences.
    private void resumeSavedActivity() {
        State.SavedActivity saved = State.global.loadActivity();
        if (saved != null) {
            if (saved.activity == State.SAVEDACTIVITY_NEWSERVER_LOGIN || saved.activity == State.SAVEDACTIVITY_LOGIN) {
                Log.d(TAG, "Resuming login process with OAuth provider " + saved.oauthProvider);
                // Inject the scanned server into the in-memory list of scanned servers, so that the
                // rest of the code can continue on as though we've just done an IP scan.
                State.global.scanner.injectServerIfNotPresent(saved.scannedServer);
                // Inform the remote app that we have a token. It must proceed with the login.
                // The next thing that will happen is the remote cyclops server will
                // us to get an IdentityToken from accounts.cyclopcam.org, by calling back
                // into requestOAuthLogin().
                // We want the remote webview to return to the same place it was at before
                // all the oauth redirect occurred. This will make the user comfortable
                // seeing a familiar screen.
                HashMap<String, String> queryParams = new HashMap<>();
                queryParams.put("have_accounts_token", "1");
                queryParams.put("provider", saved.oauthProvider);
                // On the server side, I unified /welcome and /login
                String url = "/welcome";
                //if (saved.activity == State.SAVEDACTIVITY_NEWSERVER_LOGIN)
                //    url = "/welcome";
                //else
                //    url = "/login";
                navigateToScannedLocalServer(saved.scannedServer.publicKey, url, queryParams);
            }
        }
    }
}