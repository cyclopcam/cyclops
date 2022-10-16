package org.cyclops;

import android.content.ContentValues;
import android.content.Context;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.database.sqlite.SQLiteDatabase;
import android.util.Log;

import java.lang.reflect.Array;
import java.util.ArrayList;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;

class State {
    static final State global = new State();

    static final int STATE_NEW = 0; // Record is not in database
    static final int STATE_MODIFIED = 1; // Record has been modified
    static final int STATE_NOTMODIFIED = 2; // Record has not been modified

    static final String PREF_CURRENT_SERVER_PUBLIC_KEY = "CURRENT_SERVER_PUBLIC_KEY";

    // Server is sent as JSON to appui
    // SYNC-NATCOM-SERVER
    static class Server {
        int state = STATE_NEW;
        String lanIP = "";
        String publicKey = "";
        String bearerToken = "";
        String name = "";

        Server copy() {
            Server s = new Server();
            s.state = state;
            s.lanIP = lanIP;
            s.publicKey = publicKey;
            s.bearerToken = bearerToken;
            s.name = name;
            return s;
        }
    }

    // These objects are created in MainActivity's onCreate
    Scanner scanner;
    LocalDB db;
    SharedPreferences sharedPref;

    // serversLock guards access to 'servers' and 'currentServerPublicKey'
    Lock serversLock = new ReentrantLock();
    ArrayList<Server> servers = new ArrayList<Server>();
    String currentServerPublicKey = "";

    State() {
        //Log.i("C", "Global state constructor");
    }

    void loadAll() {
        serversLock.lock();
        try {
            loadAllFromDB();
            currentServerPublicKey = sharedPref.getString(PREF_CURRENT_SERVER_PUBLIC_KEY, "");
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

    Server getCurrentServer() {
        serversLock.lock();
        try {
            for (Server s : servers) {
                if (s.publicKey.equals(currentServerPublicKey)) {
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

    void setCurrentServer(String publicKey) {
        serversLock.lock();
        try {
            Log.i("C", "setCurrentServer to " + publicKey);
            currentServerPublicKey = publicKey;
            SharedPreferences.Editor edit = sharedPref.edit();
            edit.putString(PREF_CURRENT_SERVER_PUBLIC_KEY, publicKey);
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
            String[] columns = {"publicKey", "lanIP", "bearerToken", "name"};
            Cursor c = h.query("server", columns, null, null, null, null, null);
            while (c.moveToNext()) {
                Server s = new Server();
                s.state = STATE_NOTMODIFIED;
                s.publicKey = c.getString(0);
                s.lanIP = c.getString(1);
                s.bearerToken = c.getString(2);
                s.name = c.getString(3);
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

    void addNewServer(String lanIP, String publicKey, String bearerToken, String name) {
        serversLock.lock();
        try {
            Server s = new Server();
            s.lanIP = lanIP;
            s.publicKey = publicKey;
            s.bearerToken = bearerToken;
            s.name = name;
            s.state = STATE_NEW;
            servers.add(s);
            saveServersToDB();
        } finally {
            serversLock.unlock();
        }
    }
}
