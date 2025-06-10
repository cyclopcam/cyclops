package org.cyclops;

import android.app.Activity;
import android.content.Context;
import android.net.Uri;
import android.provider.MediaStore;
import android.util.Log;
import android.webkit.ValueCallback;
import android.webkit.WebMessage;
import android.webkit.WebMessagePort;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebView;

import androidx.annotation.NonNull;
import androidx.annotation.RequiresApi;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebMessagePortCompat;
import androidx.webkit.WebViewAssetLoader;
import androidx.webkit.WebViewClientCompat;
import androidx.webkit.WebViewCompat;

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
    private final WebView webView;
    private State.Server server;
    private Map<Integer, VideoDecoder> videoDecoders;
    private Map<Integer, WebsocketPlayer> videoPlayers;
    private int nextVideoDecoderID = 1;
    private WebMessagePortCompat javaPort;
    private WebMessagePortCompat jsPort;

    RemoteWebViewClient(Main main, Activity activity, WebView webView) {
        this.main = main;
        this.activity = activity;
        this.webView = webView;
        this.videoDecoders = new HashMap<>();
        this.videoPlayers = new HashMap<>();
    }

    @Override
    public void onPageFinished(WebView view, String url) {
        Log.i("C", "Remote onPageFinished at url = '" + url + "'");
        //if (this.server != null && !this.server.publicKey.equals("") && !this.server.bearerToken.equals("")) {
        //    view.evaluateJavascript("window.cySetCredentials('" + this.server.publicKey + "' , '" + this.server.bearerToken + "')", null);
        //}
        super.onPageFinished(view, url);
        view.evaluateJavascript("window.cyActivateAppMode()", null);
        //Log.i("C", "app mode activated?");

        // Experiment with Message Ports (not yet used)
        WebMessagePortCompat[] ports = WebViewCompat.createWebMessageChannel(webView);
        javaPort = ports[0];
        jsPort = ports[1];

        Uri uri = Uri.parse(url);

        WebMessageCompat message = new WebMessageCompat("init", new WebMessagePortCompat[] { jsPort });
        WebViewCompat.postWebMessage(webView, message, uri);

        javaPort.setWebMessageCallback(new WebMessagePortCompat.WebMessageCallbackCompat() {
            @Override
            public void onMessage(WebMessagePortCompat port, WebMessageCompat message) {
                String data = message.getData();
                Log.i("C", "Message from JS: " + data);
            }
        });

        javaPort.postMessage(new WebMessageCompat("Hello from the Java side"));
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

    WebResourceResponse sendText(String msg) {
        return new WebResourceResponse("text/plain", "utf-8", 200, "OK", null, new ByteArrayInputStream(msg.getBytes(StandardCharsets.UTF_8)) );
    }

    WebResourceResponse sendError(String err) {
        // grrrr. if we send a 400 back, then the WebView doesn't get the body. Sigh!
        //return new WebResourceResponse("text/plain", "utf-8", 400, err, null, null);
        // So instead we use a string prefixed with "ERROR:" to indicate errors.
        return sendText("ERROR:" + err);
    }

    WebResourceResponse sendInt(int i) {
        return sendText(Integer.toString(i));
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
                case "/natcom/postLogin":
                    Log.i("C", "natcom/postLogin reached");
                    activity.runOnUiThread(() -> main.onPostLogin());
                    return sendOK();
                case "/natcom/networkDown":
                    Log.i("C", "natcom/networkDown reached");
                    activity.runOnUiThread(() -> main.onNetworkDown(url.getQueryParameter("errorMsg")));
                    return sendOK();
                case "/natcom/requestOAuthLogin":
                    //activity.runOnUiThread(() -> main.requestOAuthLogin(url.getQueryParameter("purpose"), url.getQueryParameter("provider")));
                    main.requestOAuthLogin(url.getQueryParameter("purpose"), url.getQueryParameter("provider"));
                    return sendOK();
                case "/natcom/wsvideo/play":
                    String wsurl = url.getQueryParameter("wsurl");
                    String codec = url.getQueryParameter("codec");
                    int width = Integer.parseInt(url.getQueryParameter("width"));
                    int height = Integer.parseInt(url.getQueryParameter("height"));
                    int id = nextVideoDecoderID;
                    nextVideoDecoderID++;
                    Log.i("C", "natcom/wsvideo/play " + wsurl + " " + codec + " " + width + " " + height);
                    try {
                        WebsocketPlayer player = new WebsocketPlayer(wsurl, codec, width, height);
                        videoPlayers.put(id, player);
                        return sendText(Integer.toString(id));
                    } catch (Exception e) {
                        return sendError(e.getMessage());
                    }
                case "/natcom/wsvideo/stop":
                    int id2 = Integer.parseInt(url.getQueryParameter("id"));
                    WebsocketPlayer player = videoPlayers.get(id2);
                    if (player != null) {
                        player.close();
                        videoPlayers.remove(id2);
                    }
                    return sendOK();
                case "/natcom/wsvideo/nextframe":
                    int id3 = Integer.parseInt(url.getQueryParameter("id"));
                    WebsocketPlayer player2 = videoPlayers.get(id3);
                    if (player2 != null) {
                        byte[] frame = player2.pollFrame();
                        if (frame != null) {
                            // Send back binary RGBA frame
                            return new WebResourceResponse("application/binary", "", 200, "OK", null, new ByteArrayInputStream(frame));
                        }
                    }
                    return new WebResourceResponse("text/plain", "utf-8", 204, "WAIT", null, null);

                /*
                case "/natcom/decoder/create":
                    String codec = url.getQueryParameter("codec");
                    int width = Integer.parseInt(url.getQueryParameter("width"));
                    int height = Integer.parseInt(url.getQueryParameter("height"));
                    int id = 0;
                    try {
                        VideoDecoder decoder = new VideoDecoder("video/" + codec, width, height);
                        id = nextVideoDecoderID;
                        nextVideoDecoderID++;
                        videoDecoders.put(id, decoder);
                    } catch (Exception e) {
                        return sendError(e.getMessage());
                    }
                    return sendInt(id);
                case "/natcom/decoder/destroy":
                    int id2 = Integer.parseInt(url.getQueryParameter("id"));
                    if (videoDecoders.containsKey(id2)) {
                        VideoDecoder decoder = videoDecoders.get(id2);
                        decoder.release();
                        videoDecoders.remove(id2);
                        return sendOK();
                    }
                    return sendOK();
                case "/natcom/decoder/packet":
                    // Packet bytes are in body
                    int id3 = Integer.parseInt(url.getQueryParameter("id"));
                    VideoDecoder decoder = videoDecoders.get(id3);
                    if (decoder != null) {

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
