package org.cyclops;

import android.net.Uri;
import android.util.Log;
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

public class LocalContentWebViewClient extends WebViewClientCompat {
    private final WebViewAssetLoader mAssetLoader;
    private final OkHttpClient client = new OkHttpClient();

    LocalContentWebViewClient(WebViewAssetLoader assetLoader) {
        mAssetLoader = assetLoader;
    }

    @Override
    public void onPageFinished(WebView view, String url) {
        super.onPageFinished(view, url);
        if (State.global.servers.size() == 0) {
            cySetMode(view, "init");
        }
    }

    void cySetMode(WebView view, String mode) {
        view.evaluateJavascript("window.cySetMode('" + mode + "')", null);
    }

    WebResourceResponse sendOK() {
        return new WebResourceResponse("text/plain", "utf-8", 200, "OK", null, null);
    }

    WebResourceResponse sendJSON(Object obj) {
        Gson gson = new Gson();
        String j = gson.toJson(obj);
        Log.i("C", "JSON is " + j);
        return new WebResourceResponse("application/json", "utf-8", 200, "OK", null, new ByteArrayInputStream(j.getBytes(StandardCharsets.UTF_8)));
    }

    @Override
    @RequiresApi(21)
    public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
        //Log.i("C", "shouldInterceptRequest " + request.getMethod() + " " + request.getUrl());

        boolean isNatCom = request.getUrl().getPath().startsWith("/natcom");
        if (isNatCom) {
            String path = request.getUrl().getPath();
            switch (path) {
                case "/natcom/scanForServers":
                    // Ignore the possibility that a scan may still be in progress.. start() will simply fail in that case,
                    // and continue it's previous scan
                    State.global.scanner.start();
                    return sendOK();
                case "/natcom/scanStatus":
                    return sendJSON(State.global.scanner.getState());
            }
        }

        /*
        //boolean isAPI = request.getUrl().getPath().startsWith("/api");
        boolean isAPI = false; // try using http://cyclops:8080 inside Javascript
        if (isAPI) {
            Log.i("C", "MUST INTERCEPT!!!");
            Uri org = request.getUrl();
            String urlStr = "http://cyclops:8080" + org.getPath();
            if (org.getEncodedQuery() != null)
                urlStr += org.getEncodedQuery();
            //Uri uri = Uri.parse(urlStr);
            Log.i("C", "Rewrite '" + org.toString() + "' to '" + urlStr + "'");

            Request.Builder reqBuilder = new Request.Builder();
            reqBuilder.url(urlStr);
            for (Map.Entry<String, String> header : request.getRequestHeaders().entrySet()) {
                reqBuilder.addHeader(header.getKey(), header.getValue());
            }
            reqBuilder.method(request.getMethod(), null);
            Request req2 = reqBuilder.build();

            try (Response resp = client.newCall(req2).execute()) {
                Log.i("C", "Forwarded request succeeded: " + resp.toString());
                HashMap<String, String> respHeaders = new HashMap<String, String>();
                for (String name : resp.headers().names()) {
                    respHeaders.put(name, resp.header(name));
                }
                String mimeType = "";
                String encoding = "";
                String contentType = resp.header("Content-Type");
                if (contentType != null) {
                    if (contentType.equals("text/plain; charset=utf-8")) {
                        mimeType = "text/plain";
                        encoding = "utf-8";
                    } else {
                        Log.i("C", "Copying contentType '" + contentType + "' to mimeType");
                        mimeType = contentType;
                    }
                }
                // I'm reading all the response bytes here, so that we can close the ResponseBody
                // and not have to worry about it anymore.
                // If we close the body now, and return resp.body().InputStream, then it's too late to read the input stream.
                ResponseBody respBody = resp.body();
                InputStream respStream = null;
                if (respBody != null) {
                    respStream = new ByteArrayInputStream(respBody.bytes());
                    //respStream = respBody.byteStream();
                }
                return new WebResourceResponse(mimeType, encoding, resp.code(), resp.message(), respHeaders, respStream);
            } catch (IOException e) {
                Log.i("C", "Forwarded request failed: " + e.toString());
            }
        }
        */

        return mAssetLoader.shouldInterceptRequest(request.getUrl());
    }

    @Override
    @SuppressWarnings("deprecation") // to support API < 21
    public WebResourceResponse shouldInterceptRequest(WebView view, String url) {
        return mAssetLoader.shouldInterceptRequest(Uri.parse(url));
    }
}
