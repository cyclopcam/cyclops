package org.cyclops;

public class Constants {
    // SYNC-SERVER-PORT
    static int ServerPort = 80; // Changing this to 8080 (Go server) or 8081 (Vue proxy) can be useful when debugging

    public static String serverLanURL(String lanIP) {
        return "http://" + lanIP + ":" + ServerPort;
    }
}
