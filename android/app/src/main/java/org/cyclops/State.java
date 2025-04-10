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
    static final int STATE_DELETE = 3; // Record must be deleted

    // SYNC-ALL-PREFS
    static final String PREF_LAST_SERVER_PUBLIC_KEY = "LAST_SERVER_PUBLIC_KEY";
    static final String PREF_ACCOUNTS_TOKEN = "ACCOUNTS_TOKEN"; // Authentication token to accounts.cyclopcam.org
    static final String PREF_SAVED_ACTIVITY = "SAVED_ACTIVITY"; // Used to remember our state

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

    static final int SAVEDACTIVITY_NEWSERVER_LOGIN = 1; // Busy logging into new server (so we are implicitly going to become the admin there, and possibly go through a setup flow)
    static final int SAVEDACTIVITY_LOGIN = 2; // Busy logging into a server that already has users on it

    // SavedActivity represents a state that our app was in before we needed to kick off some other
    // activity. This was created for the OAuth Web signin flow, where we're busy signing into
    // a new Cyclops server, and we need to invoke a Chrome Custom Tab. When our activity is
    // restarted, we need to know where we left off.
    public static class SavedActivity {
        int activity = 0; // SAVEDACTIVITY_*
        Scanner.ScannedServer scannedServer = null; // The server we were logging into
        String oauthProvider = ""; // The OAuth provider we were logging into
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
            edit.remove(PREF_ACCOUNTS_TOKEN);
            edit.remove(PREF_SAVED_ACTIVITY);
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

    // Get the authentication token to accounts.cyclopcam.org
    String getAccountsToken() {
        return sharedPref.getString(PREF_ACCOUNTS_TOKEN, "");
    }

    // Set the authentication token to accounts.cyclopcam.org
    void setAccountsToken(String token) {
        sharedPref.edit().putString(PREF_ACCOUNTS_TOKEN, token).apply();
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

    Server getAnyServer() {
        serversLock.lock();
        try {
            if (servers.size() > 0) {
                return servers.get(0);
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
            sharedPref.edit().putString(PREF_LAST_SERVER_PUBLIC_KEY, publicKey).apply();
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
            ArrayList<Server> newServerList = new ArrayList<>();
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
                } else if (s.state == STATE_MODIFIED) {
                    Log.i("C", "Updating server " + s.publicKey + " in DB");
                    ContentValues v = new ContentValues();
                    v.put("lanIP", s.lanIP);
                    v.put("bearerToken", s.bearerToken);
                    v.put("name", s.name);
                    v.put("sessionCookie", s.sessionCookie);
                    String[] args = {s.publicKey};
                    h.update("server", v, "publicKey = ?", args);
                    s.state = STATE_NOTMODIFIED;
                } else if (s.state == STATE_DELETE) {
                    Log.i("C", "Deleting server " + s.publicKey + " from DB");
                    String[] args = {s.publicKey};
                    h.delete("server", "publicKey = ?", args);
                }
                if (s.state != STATE_DELETE) {
                    newServerList.add(s);
                }
            }
            servers = newServerList;
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

    void deleteServer(String publicKey) {
        serversLock.lock();
        boolean wasLast = false;
        try {
            Server s = getServerByPublicKey(publicKey);
            if (s == null) {
                return;
            }
            s.state = STATE_DELETE;
            wasLast = lastServerPublicKey.equals(publicKey);
            saveServersToDB();
        } finally {
            serversLock.unlock();
        }

        if (wasLast) {
            setLastServer("");
        }
    }

    // Save our current activity.
    // This is used when invoking a custom chrome tab for OAuth login, so that we can save
    // where we were.
    void saveActivity(SavedActivity activity) {
        sharedPref.edit().putString(PREF_SAVED_ACTIVITY, new Gson().toJson(activity)).apply();
    }

    // Returns either the most recently saved activity, or null.
    // After loading, clears the saved activity.
    SavedActivity loadActivity() {
        String json = sharedPref.getString(PREF_SAVED_ACTIVITY, "");
        if (json.equals("")) {
            return null;
        }
        try {
            SavedActivity activity = new Gson().fromJson(json, SavedActivity.class);
            return activity;
        } catch (Exception e) {
            // Maybe the app was upgraded, and the JSON changed.
            Log.e("C", "Failed to load saved activity: " + e);
            return null;
        } finally {
            sharedPref.edit().remove(PREF_SAVED_ACTIVITY).apply();
        }
    }
}
