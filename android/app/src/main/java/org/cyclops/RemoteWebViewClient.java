package org.cyclops;

import android.app.Activity;
import android.net.Uri;
import android.util.Log;
import android.webkit.ValueCallback;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebView;

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
    private final OkHttpClient client = new OkHttpClient();
    private final Main main;

    RemoteWebViewClient(Main main) {
        this.main = main;
    }

    @Override
    public void onPageFinished(WebView view, String url) {
        super.onPageFinished(view, url);
    }

    void cyBack(WebView view) {
        view.evaluateJavascript("window.cyBack()", new ValueCallback<String>() {
            @Override
            public void onReceiveValue(String value) {
                //Log.i("C", "cyBack response " + value);
                if (!value.equals("true")) {
                    main.webViewBackFailed();
                }
            }
        });
    }

    WebResourceResponse sendOK() {
        return new WebResourceResponse("text/plain", "utf-8", 200, "OK", null, null);
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
