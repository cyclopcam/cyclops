package org.cyclops;

import android.content.ContentValues;
import android.content.Context;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.database.sqlite.SQLiteDatabase;
import android.util.Base64;
import android.util.Log;
import android.webkit.WebResourceResponse;

import com.google.gson.Gson;

import java.io.IOException;
import java.lang.reflect.Array;
import java.util.ArrayList;
//import java.util.Base64;
import java.util.HashMap;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;

class State {
    static final State global = new State();

    static final int STATE_NEW = 0; // Record is not in database
    static final int STATE_MODIFIED = 1; // Record has been modified
    static final int STATE_NOTMODIFIED = 2; // Record has not been modified

    // SYNC-ALL-PREFS
    static final String PREF_LAST_SERVER_PUBLIC_KEY = "LAST_SERVER_PUBLIC_KEY";

    // Server is sent as JSON to appui
    // SYNC-NATCOM-SERVER
    static class Server {
        int state = STATE_NEW;
        String lanIP = "";
        String publicKey = "";
        String bearerToken = "";
        String name = "";
        String sessionCookie = "";

        Server copy() {
            Server s = new Server();
            s.state = state;
            s.lanIP = lanIP;
            s.publicKey = publicKey;
            s.bearerToken = bearerToken;
            s.name = name;
            s.sessionCookie = sessionCookie;
            return s;
        }
    }

    // These objects are created in MainActivity's onCreate
    Scanner scanner;
    LocalDB db;
    SharedPreferences sharedPref;

    // serversLock guards access to 'servers' and 'lastServerPublicKey'
    Lock serversLock = new ReentrantLock();
    ArrayList<Server> servers = new ArrayList<Server>();
    String lastServerPublicKey = "";

    private final HttpClient client = new HttpClient();

    State() {
        //Log.i("C", "Global state constructor");
    }

    // This was built for debugging, to reset an application install to it's initial just-installed state
    void resetAllState() {
        Log.i("C", "Resetting all state");
        serversLock.lock();
        try {
            // SYNC-ALL-PREFS
            SharedPreferences.Editor edit = sharedPref.edit();
            edit.remove(PREF_LAST_SERVER_PUBLIC_KEY);
            edit.apply();

            SQLiteDatabase h = db.getWritableDatabase();
            h.delete("server", "", null);

            lastServerPublicKey = "";
            servers = new ArrayList<Server>();
        } finally {
            serversLock.unlock();
        }
    }

    void loadAll() {
        serversLock.lock();
        try {
            loadAllFromDB();
            lastServerPublicKey = sharedPref.getString(PREF_LAST_SERVER_PUBLIC_KEY, "");
        } finally {
            serversLock.unlock();
        }
    }

    // Returns a deep copy of the servers list
    ArrayList<Server> getServersCopy() {
        serversLock.lock();
        try {
            ArrayList<Server> copy = new ArrayList<>();
            for (Server s : servers) {
                copy.add(s.copy());
            }
            return copy;
        } finally {
            serversLock.unlock();
        }
    }

    Server getLastServer() {
        serversLock.lock();
        try {
            for (Server s : servers) {
                if (s.publicKey.equals(lastServerPublicKey)) {
                    return s;
                }
            }
        } finally {
            serversLock.unlock();
        }
        return null;
    }

    Server getServerCopyByPublicKey(String publicKey) {
        try {
            serversLock.lock();
            Server s = getServerByPublicKey(publicKey);
            if (s == null) {
                return s;
            }
            return s.copy();
        } finally {
            serversLock.unlock();
        }
    }

    Server getServerByPublicKey(String publicKey) {
        serversLock.lock();
        try {
            for (Server s : servers) {
                if (s.publicKey.equals(publicKey)) {
                    return s;
                }
            }
            return null;
        } finally {
            serversLock.unlock();
        }
    }

    void setLastServer(String publicKey) {
        serversLock.lock();
        try {
            Log.i("C", "setLastServer to " + publicKey);
            lastServerPublicKey = publicKey;
            SharedPreferences.Editor edit = sharedPref.edit();
            edit.putString(PREF_LAST_SERVER_PUBLIC_KEY, publicKey);
            edit.apply();
        } finally {
            serversLock.unlock();
        }
    }

    void setServerProperty(String publicKey, String key, String value) {
        serversLock.lock();
        try {
            Log.i("C", "setServerProperty " + key + " : " + value);
            Server s = getServerByPublicKey(publicKey);
            if (s == null) {
                return;
            }
            switch (key) {
                case "name":
                    s.name = value;
                    s.state = STATE_MODIFIED;
                    break;
                case "sessionCookie":
                    s.sessionCookie = value;
                    s.state = STATE_MODIFIED;
                    break;
                default:
                    Log.e("C", "Unknown property '" + key + "'");
                    return;
            }
            saveServersToDB();
        } finally {
            serversLock.unlock();
        }
    }

    private void loadAllFromDB() {
        serversLock.lock();
        try {
            servers.clear();
            SQLiteDatabase h = db.getReadableDatabase();
            String[] columns = {"publicKey", "lanIP", "bearerToken", "name", "sessionCookie"};
            Cursor c = h.query("server", columns, null, null, null, null, null);
            while (c.moveToNext()) {
                Server s = new Server();
                s.state = STATE_NOTMODIFIED;
                s.publicKey = c.getString(0);
                s.lanIP = c.getString(1);
                s.bearerToken = c.getString(2);
                s.name = c.getString(3);
                s.sessionCookie = c.getString(4);
                servers.add(s);
            }
            Log.i("C", "Loaded " + servers.size() + " servers from DB");
            c.close();
        } finally {
            serversLock.unlock();
        }
    }

    private void saveServersToDB() {
        serversLock.lock();
        try {
            SQLiteDatabase h = db.getWritableDatabase();
            // Update existing
            for (Server s : servers) {
                if (s.state == STATE_MODIFIED) {
                    Log.i("C", "Updating server " + s.publicKey + " in DB");
                    ContentValues v = new ContentValues();
                    v.put("lanIP", s.lanIP);
                    v.put("bearerToken", s.bearerToken);
                    v.put("name", s.name);
                    v.put("sessionCookie", s.sessionCookie);
                    String[] args = {s.publicKey};
                    h.update("server", v, "publicKey = ?", args);
                    s.state = STATE_NOTMODIFIED;
                }
            }
            // Insert new
            for (Server s : servers) {
                if (s.state == STATE_NEW) {
                    Log.i("C", "Adding server " + s.publicKey + " to DB");
                    ContentValues v = new ContentValues();
                    v.put("publicKey", s.publicKey);
                    v.put("lanIP", s.lanIP);
                    v.put("bearerToken", s.bearerToken);
                    v.put("name", s.name);
                    v.put("sessionCookie", s.sessionCookie);
                    h.insert("server", null, v);
                    s.state = STATE_NOTMODIFIED;
                }
            }
        } finally {
            serversLock.unlock();
        }
    }

    void close() {
        db.close();
        db = null;
    }

    void addOrUpdateServer(String lanIP, String publicKey, String bearerToken, String name, String sessionCookie) {
        serversLock.lock();
        try {
            Server s = getServerByPublicKey(publicKey);
            if (s == null) {
                Log.i("C", "Adding new server " + publicKey + " (" + name + ")");
                s = new Server();
                servers.add(s);
                s.state = STATE_NEW;
                s.publicKey = publicKey;
                s.name = name;
            } else {
                s.state = STATE_MODIFIED;
            }
            s.lanIP = lanIP;
            s.bearerToken = bearerToken;
            s.sessionCookie = sessionCookie;
            saveServersToDB();
        } finally {
            serversLock.unlock();
        }
    }

    //static class LoginResult {
    //    String error;
    //    String token;
    //}

    // Use our bearer token to perform a cookie-based login, and set the cookie for our webviews
    //void recreateSession(String publicKey, boolean isProxy) {
    //}

    // The code below should work.. but I decided to keep logins on the Typescript side.
    // There's no benefit to performing logins here.
    /*
    // Returns an empty string on success, or an error message on failure
    String login(String url, String publicKey, String username, String password) {
        // Talk to server
        LoginResult lr = performLogin(url, publicKey, username, password);
        if (lr.error != null) {
            return lr.error;
        }

        // Save session token
        serversLock.lock();
        try {
            Server s = getServerByPublicKey(publicKey);
            if (s == null) {
                s = new Server();
                s.publicKey = publicKey;
                s.state = STATE_NEW;
            }
            s.bearerToken = lr.token;
            servers.add(s);
            saveServersToDB();
        } finally {
            serversLock.unlock();
        }

        return "";
    }

    // Returns an empty string on success, or an error message on failure
    LoginResult performLogin(String url, String publicKey, String username, String password) {
        HashMap<String, String> headers = new HashMap<>();
        headers.put("Authorization", "BASIC " + Base64.encodeToString((username + ":" + password).getBytes(), 0));
        HttpClient.Response resp = client.POST(url, headers);
        LoginResult result = new LoginResult();
        if (resp.Error != null) {
            result.error = resp.Error;
        } else {
            ResponseBody body = resp.Resp.body();
            if (resp.Resp.code() == 200 && body != null) {
                Gson gson = new Gson();
                JSAPI.LoginResponseJSON v = gson.fromJson(body.charStream(), JSAPI.LoginResponseJSON.class);
                result.token = v.bearerToken;
            } else {
                if (body != null) {
                    result.error = body.toString();
                } else {
                    result.error = "Error " + resp.Resp.code();
                }
            }
            if (body != null) {
                body.close();
            }
        }
        return result;
    }
     */
}
