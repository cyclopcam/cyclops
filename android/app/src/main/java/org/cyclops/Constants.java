package org.cyclops;

public class Constants {
    // SYNC-SERVER-PORT
    static int ServerPort = 8080;

    public static String serverLanURL(String lanIP) {
        return "http://" + lanIP + ":" + ServerPort;
    }
}
