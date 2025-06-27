package org.cyclops;

import android.provider.MediaStore;
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
    // If resp is not null, and it had a body, then it will have been read into Body
    // You do not need to close the response, as it is closed in this class.
    static class Response {
        String Error;
        okhttp3.Response Resp; // Resp.body has already been read into Body

        // If Resp was not null, and the body was not null, this is the body.
        String Body;

        // If Error is not null, then this is null.
        // If Body is not null, then this is Body.
        // If Body is null, then this is the status code and message (eg "404 Not Found", or "200 OK")
        // Basically, you'll usually use this as an error message, if Resp.code() != 200.
        String BodyOrStatusString;

        Response(String error) {
            Error = error;
        }
        Response(okhttp3.Response resp, String body) {
            Resp = resp;
            Body = body;
            if (body != null) {
                BodyOrStatusString = body;
            } else {
                BodyOrStatusString = resp.code() + " " + resp.message();
            }
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
    Response POST(String url, HashMap<String,String> headers, MediaType bodyContentType, byte[] body) {
        return Do("POST", url, headers, bodyContentType, body);
    }

    Response Do(String method, String url, HashMap<String,String> headers) {
        return Do(method, url, headers, null, null);
    }

    Response Do(String method, String url, HashMap<String,String> headers, MediaType bodyContentType, byte[] body) {
        Request.Builder builder = new Request.Builder();
        //try {
            builder.url(url);
            if (headers != null) {
                for (String key : headers.keySet()) {
                    builder.addHeader(key, headers.get(key));
                }
            }
            if (method.equals("POST")) {
                builder.method(method, new RequestBody() {
                    @Nullable
                    @Override
                    public MediaType contentType() {
                        return bodyContentType;
                    }

                    @Override
                    public void writeTo(@NonNull BufferedSink sink) throws IOException {
                        if (body != null) {
                            sink.write(body);
                        }
                    }
                });
            } else {
                builder.method(method, null);
            }

            try (okhttp3.Response resp = client.newCall(builder.build()).execute()) {
                if (resp.body() != null) {
                    String bodyString = resp.body().string();
                    return new Response(resp, bodyString);
                } else {
                    return new Response(resp, null);
                }
            } catch (IOException e) {
                Log.e("C", "Failed to read from " + url + ": " + e.toString());
                return new Response("Failed to read from " + url + ": " + e.toString());
            }
            //okhttp3.Response resp = client.newCall(builder.build()).execute();
            //if (resp.body() != null) {
            //    try {
            //        String bodyString = resp.body().string();
            //        resp.close();
            //        return new Response(resp, bodyString);
            //    } catch (IOException e) {
            //        resp.close();
            //        Log.e("C", "Failed to read response body: " + e.toString());
            //        return new Response("Failed to read response body: " + e.toString());
            //    }
            //} else {
            //    return new Response(resp, null);
            //}
        //} catch (IOException e) {
        //    Log.e("C", "Failed to contact " + url + ": " + e.toString());
        //    return new Response(e.toString());
        //}
    }

}
