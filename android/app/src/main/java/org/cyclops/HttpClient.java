package org.cyclops;

import android.util.Log;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;

import java.io.IOException;
import java.io.UnsupportedEncodingException;
import java.net.URLEncoder;
import java.util.HashMap;
import java.util.logging.Level;
import java.util.logging.Logger;

import kotlin.text.Charsets;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import okio.BufferedSink;

// HttpClient is a wrapper around okhttp3
public class HttpClient {
    public final OkHttpClient client;

    HttpClient() {
        // Can be used to get stack traces if we're leaking response bodies
        //Logger.getLogger(OkHttpClient.class.getName()).setLevel(Level.FINE);

        this.client = new OkHttpClient();
    }

    HttpClient(OkHttpClient client) {
        this.client = client;
    }

    // Either a network error, or a response.
    // If Error is null, then Resp is not null.
    // If Resp is null, then Error is not null.
    static class Response {
        String Error;
        okhttp3.Response Resp;

        Response(String error) {
            Error = error;
        }
        Response(okhttp3.Response resp) {
            Resp = resp;
        }
    }

    String encodeQuery(String k) {
        try {
            return URLEncoder.encode(k, Charsets.UTF_8.name());
        } catch (UnsupportedEncodingException e) {
            e.printStackTrace();
            return "";
        }
    }

    String encodeQuery(String k1, String v1) {
        return encodeQuery(k1) + "=" + encodeQuery(v1);
    }

    String encodeQuery(String k1, String v1, String k2, String v2) {
        return encodeQuery(k1) + "=" + encodeQuery(v1) + "&" +
                encodeQuery(k2) + "=" + encodeQuery(v2);
    }

    Response GET(String url, HashMap<String,String> headers) {
        return Do("GET", url, headers);
    }
    Response POST(String url, HashMap<String,String> headers) {
        return Do("POST", url, headers);
    }

    Response Do(String method, String url, HashMap<String,String> headers) {
        Request.Builder builder = new Request.Builder();
        try {
            builder.url(url);
            if (headers != null) {
                for (String key : headers.keySet()) {
                    builder.addHeader(key, headers.get(key));
                }
            }
            if (method.equals("POST")) {
                // Create an empty body for POST requests
                builder.method(method, new RequestBody() {
                    @Nullable
                    @Override
                    public MediaType contentType() {
                        return null;
                    }

                    @Override
                    public void writeTo(@NonNull BufferedSink sink) throws IOException {
                    }
                });
            } else {
                builder.method(method, null);
            }
            return new Response(client.newCall(builder.build()).execute());
        } catch (IOException e) {
            Log.e("C", "Failed to contact " + url + ": " + e.toString());
            return new Response(e.toString());
        }
    }

}
