package org.cyclops;

import static org.cyclops.Constants.ServerPort;

import android.content.Context;

import java.io.IOException;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;

public class Connector {
    // SYNC-CONNECT-RESPONSE
    static class NewConnectionResponse {
        String state; // "n" | "o" | "e"; // new, old, error
        String error;
    }

    Context context;
    private final OkHttpClient client = new OkHttpClient();

    Connector(Context context) {
        this.context = context;
    }

    NewConnectionResponse newConnection(String ip) {
        NewConnectionResponse result = new NewConnectionResponse();
        Request req = new Request.Builder().url("http://" + ip + ":" + ServerPort + "/api/auth/hasAdmin").build();
        try {
            Response resp = client.newCall(req).execute();
            String rs = resp.body().string();
            if (rs.equals("true")) {
                result.state = "o"; // admin user already created, so this is an old server
            } else {
                result.state = "n"; // no admin user, so this is a new server
            }
        } catch (IOException e) {
            result.state = "e";
            result.error = e.toString();
        }
        return result;
    }
}
