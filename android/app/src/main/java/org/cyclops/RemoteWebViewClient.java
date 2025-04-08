package org.cyclops;

import android.app.Activity;
import android.content.Context;
import android.net.Uri;
import android.util.Log;
import android.webkit.ValueCallback;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebView;

import androidx.annotation.NonNull;
import androidx.annotation.RequiresApi;
import androidx.webkit.WebViewAssetLoader;
import androidx.webkit.WebViewClientCompat;

import com.google.gson.Gson;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.StringBufferInputStream;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;

public class RemoteWebViewClient extends WebViewClientCompat {
    //private final OkHttpClient client = new OkHttpClient();
    private final Main main;
    private final Activity activity;
    private State.Server server;

    RemoteWebViewClient(Main main, Activity activity) {
        this.main = main;
        this.activity = activity;
    }

    @Override
    public void onPageFinished(WebView view, String url) {
        Log.i("C", "Remote onPageFinished");
        //if (this.server != null && !this.server.publicKey.equals("") && !this.server.bearerToken.equals("")) {
        //    view.evaluateJavascript("window.cySetCredentials('" + this.server.publicKey + "' , '" + this.server.bearerToken + "')", null);
        //}
        super.onPageFinished(view, url);
        view.evaluateJavascript("window.cyActivateAppMode()", null);
        //Log.i("C", "app mode activated?");
    }

    @Override
    public void onReceivedHttpError(@NonNull WebView view, @NonNull WebResourceRequest request, @NonNull WebResourceResponse errorResponse) {
        super.onReceivedHttpError(view, request, errorResponse);
        String errorResponseBody = "";
        try {
            InputStream is = errorResponse.getData();
            if (is != null) {
                byte[] buffer = new byte[is.available()];
                is.read(buffer);
                errorResponseBody = new String(buffer);
            }
        } catch (IOException e) {
            //e.printStackTrace();
        }
        Map<String, String> responseHeaders = errorResponse.getResponseHeaders();
        if (responseHeaders.getOrDefault("x-cyclops-proxy-status", "").equals("SERVER_NOT_FOUND")) {
            Log.i("C", "Proxy has no record of our server");
            // TODO:
        }
        // TODO: When errorResponse.getStatusCode() returns a 502, the most likely cause is that the
        // VPN isn't working, so perhaps their server is down. We should show a decent error page
        // in this situation.
        Log.i("C", "Remote onReceivedHttpError: " + errorResponse.getStatusCode() + " " + errorResponse.getReasonPhrase() + " >> " + errorResponseBody);
    }

    void setServer(State.Server server) {
        this.server = server;
    }

    void cyBack(WebView view) {
        view.evaluateJavascript("window.cyBack()", new ValueCallback<String>() {
            @Override
            public void onReceiveValue(String value) {
                //Log.i("C", "cyBack response " + value);
                if (!value.equals("true")) {
                    activity.runOnUiThread(main::webViewBackFailed);
                }
            }
        });
    }

    // The client recognizes messages that start with "ERROR:"
    // Anything else is a normal progress message.
    void cySetProgressMessage(WebView view, String msg) {
        activity.runOnUiThread(() -> { view.evaluateJavascript("window.cySetProgressMessage('" + msg + "')", null); });
    }

    void cySetIdentityToken(WebView view, String token) {
        String js = "window.cySetIdentityToken('" + token + "')";
        Log.i("C", "DEBUG:" + js);
        activity.runOnUiThread(() -> { view.evaluateJavascript(js, null); });
    }

    WebResourceResponse sendOK() {
        return new WebResourceResponse("text/plain", "utf-8", 200, "OK", null, null);
    }

    WebResourceResponse sendError(String err) {
        return new WebResourceResponse("text/plain", "utf-8", 400, err, null, null);
    }

    WebResourceResponse sendJSON(Object obj) {
        Gson gson = new Gson();
        String j = gson.toJson(obj);
        //Log.i("C", "JSON is " + j);
        return new WebResourceResponse("application/json", "utf-8", 200, "OK", null, new ByteArrayInputStream(j.getBytes(StandardCharsets.UTF_8)));
    }

    @Override
    @RequiresApi(21)
    public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
        //Log.i("C", "shouldInterceptRequest " + request.getMethod() + " " + request.getUrl());

        Uri url = request.getUrl();
        boolean isNatCom = url.getPath().startsWith("/natcom");
        if (isNatCom) {
            String path = url.getPath();
            switch (path) {
                case "/natcom/hello":
                    return sendOK();
                case "/natcom/login":
                    Log.i("C", "natcom/login reached");
                    activity.runOnUiThread(() -> main.onLogin(url.getQueryParameter("bearerToken"), url.getQueryParameter("sessionCookie")));
                    return sendOK();
                case "/natcom/networkDown":
                    Log.i("C", "natcom/networkDown reached");
                    activity.runOnUiThread(() -> main.onNetworkDown(url.getQueryParameter("errorMsg")));
                    return sendOK();
                case "/natcom/requestOAuthLogin":
                    //activity.runOnUiThread(() -> main.requestOAuthLogin(url.getQueryParameter("purpose"), url.getQueryParameter("provider")));
                    main.requestOAuthLogin(url.getQueryParameter("purpose"), url.getQueryParameter("provider"));
                    return sendOK();
            }
        }

        return null;
    }

    @Override
    @SuppressWarnings("deprecation") // to support API < 21
    public WebResourceResponse shouldInterceptRequest(WebView view, String url) {
        return null;
    }
}
