package org.cyclops;

import android.content.Context;
import android.net.wifi.WifiInfo;
import android.net.wifi.WifiManager;
import android.util.Log;

import com.google.gson.Gson;

import java.io.IOException;
import java.util.ArrayList;
import java.util.concurrent.TimeUnit;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;

// Scan LAN for Cyclops servers
public class Scanner {
    // State is marshalled directly into a JSON response
    // SYNC-SCAN-STATE
    static class State {
        String error = "";
        String phoneIP = "";
        String status = "i"; // i:initial, b:busy, e:error, s:success
        ArrayList<String> servers = new ArrayList<String>();
        int nScanned = 0;

        synchronized void addServer(String ip) {
            servers.add(ip);
        }
        synchronized void setPhoneIP(String ip) {
            phoneIP = ip;
        }
        synchronized void setError(String err) {
            error = err;
        }
        synchronized void setStatus(String s) {
            status = s;
        }
        synchronized void reset() {
            error = "";
            nScanned = 0;
            servers = new ArrayList<>();
        }
        synchronized void incScanned() {
            nScanned++;
        }
        synchronized ArrayList<String> getServers() {
            return (ArrayList<String>) servers.clone();
        }
        synchronized String getError() {
            return error;
        }
        synchronized String getStatus() {
            return status;
        }
        synchronized int getnScanned() {
            return nScanned;
        }
        synchronized State copy() {
            State c = new State();
            c.error = error;
            c.phoneIP = phoneIP;
            c.status = status;
            c.nScanned = nScanned;
            c.servers = (ArrayList<String>) servers.clone();
            return c;
        }
    }

    private Context context;
    private State state = new State();

    Scanner(Context context) {
        this.context = context;
    }

    // returns false if a scan is already busy
    boolean start() {
        if (state.getStatus().equals("b")) {
            return false;
        }

        state.reset();
        state.setStatus("b");

        Scanner self = this;
        new Thread(new Runnable() {
            public void run() {
                Scanner.runScan(self);
            }
        }).start();
        return true;
    }

    // Return a copy of the state
    State getState() {
        return state.copy();
    }

    static int getWifiIPAddress(Context context) {
        WifiManager wifiMgr = (WifiManager) context.getApplicationContext().getSystemService(Context.WIFI_SERVICE);
        WifiInfo wifiInfo = wifiMgr.getConnectionInfo();
        if (wifiInfo == null) {
            return 0;
        }
        return wifiInfo.getIpAddress();
    }

    static String formatIP(int ip) {
        return Integer.toString(ip & 0xff) + "." + Integer.toString((ip >>> 8) & 0xff) + "." + Integer.toString((ip >>> 16) & 0xff) + "." + Integer.toString((ip >>> 24) & 0xff);
    }

    static int setLowestByteOfIP(int ip, int v) {
        // because we're little endian, last part of IP is highest byte
        return (ip & 0x00ffffff) | (v << 24);
    }

    static void runScan(Scanner scanner) {
        Log.i("C", "Getting local IP address");
        int phoneIP = getWifiIPAddress(scanner.context);
        if (phoneIP == 0) {
            scanner.state.setError("No WiFi address found");
            scanner.state.setStatus("d");
            return;
        }
        Log.i("C", "Local IP is " + formatIP(phoneIP));
        scanner.state.setPhoneIP("Phone IP: " + formatIP(phoneIP));
        ArrayList<String> ipAddresses = new ArrayList<>();
        for (int i = 1; i < 255; i++) {
            int scanIP = setLowestByteOfIP(phoneIP, i);
            if (scanIP != phoneIP) {
                ipAddresses.add(formatIP(scanIP));
            }
        }
        Log.i("C", "Scanning from " + ipAddresses.get(0) + " to " + ipAddresses.get(ipAddresses.size() - 1));
        Log.i("C", "Launching scanner threads");
        int nThreads = 8;
        ArrayList<Thread> threads = new ArrayList<>();
        int nextIPIdx = 0;
        for (int i = 0; i < nThreads; i++) {
            // carve a slice of the addresses for this thread
            int upTo = (i + 1) * ipAddresses.size() / nThreads;
            ArrayList<String> chunk = new ArrayList<>();
            for (; nextIPIdx < upTo; nextIPIdx++) {
                chunk.add(ipAddresses.get(nextIPIdx));
            }
            Thread t = new Thread(new Runnable() {
                public void run() {
                    Scanner.scanAddresses(chunk, scanner.state);
                }
            });
            t.start();
            threads.add(t);
        }
        Log.i("C", "Waiting for scanner threads");
        for (int i = 0; i < nThreads; i++) {
            try {
                threads.get(i).join();
            } catch (InterruptedException e) {
            }
        }
        Log.i("C", "Scanner threads finished");
        scanner.state.setStatus("d");
    }

    static void scanAddresses(ArrayList<String> ipAddresses, State state) {
        OkHttpClient client = new OkHttpClient.Builder()
                .callTimeout(200, TimeUnit.MILLISECONDS)
                .build();
        Gson gson = new Gson();
        for (String scanIP : ipAddresses) {
            String url = "http://" + scanIP + ":" + Constants.ServerPort + "/api/ping";
            //Log.i("C", "Scanning " + url);
            Request req = new Request.Builder().url(url).build();
            try {
                Response resp = client.newCall(req).execute();
                ResponseBody body = resp.body();
                if (resp.code() == 200 && body != null) {
                    JSAPI.PingResponseJSON ping = gson.fromJson(body.string(), JSAPI.PingResponseJSON.class);
                    if (ping.greeting.equals("I am Cyclops")) {
                        Log.i("C", "Found Cyclops server at " + scanIP);
                        state.addServer(scanIP + " (" + ping.hostname + ")");
                    }
                }
                if (body != null) {
                    body.close();
                }
            } catch (IOException e) {
            }
            state.incScanned();
            //Log.i("C", "after inc, nScanned = " + state.getnScanned());
        }
    }

}
