package org.cyclops;

import android.content.ContentValues;
import android.content.Context;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.database.sqlite.SQLiteDatabase;
import android.util.Log;

import java.util.ArrayList;

class State {
    static final State global = new State();

    static final int STATE_NEW = 0; // Record is not in database
    static final int STATE_MODIFIED = 1; // Record has been modified
    static final int STATE_NOTMODIFIED = 2; // Record has not been modified

    static final String PREF_CURRENT_SERVER_PUBLIC_KEY = "CURRENT_SERVER_PUBLIC_KEY";

    class Server {
        int state = STATE_NEW;
        String lanIP = "";
        String publicKey = "";
        String bearerToken = "";

        Server copy() {
            Server s = new Server();
            s.state = state;
            s.lanIP = lanIP;
            s.publicKey = publicKey;
            s.bearerToken = bearerToken;
            return s;
        }
    }

    // These objects are created in MainActivity's onCreate
    Scanner scanner;
    Connector connector;
    LocalDB db;
    SharedPreferences sharedPref;

    ArrayList<Server> servers = new ArrayList<Server>();
    String currentServerPublicKey = "";

    State() {
        //Log.i("C", "Global state constructor");
    }

    void loadAll() {
        loadAllFromDB();
        currentServerPublicKey = sharedPref.getString(PREF_CURRENT_SERVER_PUBLIC_KEY, "");
    }

    Server getCurrentServer() {
        for (Server s : servers) {
            if (s.publicKey.equals(currentServerPublicKey)) {
                return s;
            }
        }
        return null;
    }

    void setCurrentServer(String publicKey) {
        Log.i("C", "setCurrentServer to " + publicKey);
        currentServerPublicKey = publicKey;
        SharedPreferences.Editor edit = sharedPref.edit();
        edit.putString(PREF_CURRENT_SERVER_PUBLIC_KEY, publicKey);
        edit.apply();
    }

    private void loadAllFromDB() {
        servers.clear();
        SQLiteDatabase h = db.getReadableDatabase();
        String[] columns = {"publicKey", "lanIP", "bearerToken"};
        Cursor c = h.query("server", columns, null, null, null, null, null);
        while (c.moveToNext()) {
            Server s = new Server();
            s.state = STATE_NOTMODIFIED;
            s.publicKey = c.getString(0);
            s.lanIP = c.getString(1);
            s.bearerToken = c.getString(2);
            servers.add(s);
        }
        Log.i("C", "Loaded " + servers.size() + " servers from DB");
        c.close();
    }

    private void saveServersToDB() {
        SQLiteDatabase h = db.getWritableDatabase();
        // Update existing
        for (Server s : servers) {
            if (s.state == STATE_MODIFIED) {
                Log.i("C", "Updating server " + s.publicKey + " in DB");
                ContentValues v = new ContentValues();
                v.put("lanIP", s.lanIP);
                v.put("bearerToken", s.bearerToken);
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
                h.insert("server", null, v);
                s.state = STATE_NOTMODIFIED;
            }
        }
    }

    void close() {
        db.close();
        db = null;
    }

    void addNewServer(String lanIP, String publicKey, String bearerToken) {
        Server s = new Server();
        s.lanIP = lanIP;
        s.publicKey = publicKey;
        s.bearerToken = bearerToken;
        s.state = STATE_NEW;
        servers.add(s);
        saveServersToDB();
    }
}
