package org.cyclops;

import android.app.Activity;
import android.content.Context;
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
import com.google.gson.GsonBuilder;

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
    private final WebViewAssetLoader assetLoader;
    private final OkHttpClient client = new OkHttpClient();
    private final Main main;
    private final Activity activity;

    LocalContentWebViewClient(WebViewAssetLoader assetLoader, Main main, Activity activity) {
        this.assetLoader = assetLoader;
        this.main = main;
        this.activity = activity;
    }

    @Override
    public void onPageFinished(WebView view, String url) {
        super.onPageFinished(view, url);
        if (State.global.servers.size() == 0) {
            cySetRoute(view, "rtInit");
        }
    }

    void cySetRoute(WebView view, String route) {
        view.evaluateJavascript("window.cySetRoute('" + route + "')", null);
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

    WebResourceResponse sendJSON(Object obj) {
        // By using serializeNulls(), we get blank strings coming through as blank strings (instead of being omitted)
        Gson gson = new GsonBuilder().serializeNulls().create();
        // hmm no... blank strings can be nulls. perhaps more confusing. hmmmm not sure
        //Gson gson = new Gson();
        String j = gson.toJson(obj);
        Log.i("C", "JSON is " + j);
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
                case "/natcom/scanForServers":
                    // Ignore the possibility that a scan may still be in progress.. start() will simply fail in that case,
                    // and continue it's previous scan
                    State.global.scanner.start();
                    return sendOK();
                case "/natcom/scanStatus":
                    return sendJSON(State.global.scanner.getState());
                case "/natcom/forward":
                    return forward(request);
                case "/natcom/navigateToScannedLocalServer":
                    activity.runOnUiThread(() -> main.navigateToServer(url.getQueryParameter("url"), true, null));
                    return sendOK();
                case "/natcom/switchToRegisteredServer":
                    activity.runOnUiThread(() -> main.switchToServerByPublicKey(url.getQueryParameter("publicKey")));
                    return sendOK();
                case "/natcom/getCurrentServer":
                    State.Server s = State.global.getCurrentServer();
                    if (s == null) {
                        s = new State.Server();
                    }
                    return sendJSON(s);
                case "/natcom/getRegisteredServers":
                    return sendJSON(State.global.getServersCopy());
                case "/natcom/showMenu":
                    activity.runOnUiThread(() -> main.showMenu(url.getQueryParameter("show").equals("1")));
                    return sendOK();
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

        return assetLoader.shouldInterceptRequest(request.getUrl());
    }

    // Forward an arbitrary request to an arbitrary server.
    // The giant deficiency here, which is an absurd shortcoming of this WebView infrastructure,
    // is that the Request Body is discarded. There is no way of accessing it.
    // So any request data needs to be encoded in the URL or headers.
    public WebResourceResponse forward(WebResourceRequest request) {
        Request.Builder builder = new Request.Builder();
        try {
            // The caller specifies the complete target url with the url=... query parameter
            builder.method(request.getMethod(), null).url(request.getUrl().getQueryParameter("url"));
        } catch (IllegalArgumentException e) {
            return new WebResourceResponse("text/plain", "utf-8", 500, e.toString(), null, null);
        }

        // Copy request headers
        Map<String, String> headers = request.getRequestHeaders();
        for (String key : headers.keySet()) {
            if (key.startsWith("X-Forward-")) {
                builder.addHeader(key.substring(10), headers.get(key));
            }
        }

        try {
            Response resp = client.newCall(builder.build()).execute();
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
                } else if (contentType.equals("application/json; charset=utf-8")) {
                    mimeType = "application/json";
                    encoding = "utf-8";
                } else {
                    //Log.i("C", "Copying contentType '" + contentType + "' to mimeType");
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
            }
            return new WebResourceResponse(mimeType, encoding, resp.code(), resp.message(), respHeaders, respStream);
        } catch (IOException e) {
            return new WebResourceResponse("text/plain", "utf-8", 500, e.toString(), null, null);
        }
    }

    @Override
    @SuppressWarnings("deprecation") // to support API < 21
    public WebResourceResponse shouldInterceptRequest(WebView view, String url) {
        return assetLoader.shouldInterceptRequest(Uri.parse(url));
    }
}
