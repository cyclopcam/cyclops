package org.cyclops;

public class JSAPI {
    public static class ScanResponseJSON {
        String message = "";
        String state = "";
    }
    public static class PingResponseJSON {
        String greeting = "";
        String hostname = "";
        String publicKey = "";
        long time = 0;
    }
    public static class ScreenParamsJSON {
        int contentHeight = 0;
    }
}
