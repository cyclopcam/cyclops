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
        //if (this.server != null && !this.server.publicKey.equals("") && !this.server.bearerToken.equals("")) {
        //    view.evaluateJavascript("window.cySetCredentials('" + this.server.publicKey + "' , '" + this.server.bearerToken + "')", null);
        //}
        super.onPageFinished(view, url);
    }

    @Override
    public void onReceivedHttpError(@NonNull WebView view, @NonNull WebResourceRequest request, @NonNull WebResourceResponse errorResponse) {
        super.onReceivedHttpError(view, request, errorResponse);
        Log.i("C", "Remote onReceivedHttpError: " + errorResponse.toString());
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
                    activity.runOnUiThread(() -> main.onLogin(url.getQueryParameter("bearerToken"), url.getQueryParameter("sessionCookie")));
                    return sendOK();
                /*
                case "/natcom/login2":
                    String err = State.global.login(url.getQueryParameter("url"), url.getQueryParameter("publicKey"), url.getQueryParameter("username"), url.getQueryParameter("password"));
                    if (err.equals("")) {
                        return sendOK();
                    } else {
                        return sendError(err);
                    }
                 */
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
