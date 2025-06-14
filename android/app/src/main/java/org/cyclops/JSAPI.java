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
    // SYNC-LOGIN-RESPONSE-JSON
    public static class LoginResponseJSON {
        String bearerToken = "";
        boolean needRestart = false;
    }

    // SYNC-KEYS-RESPONSE-JSON
    public static class KeysResponseJSON {
        String publicKey;
        String proof; // HMAC[SHA256](sharedSecret, challenge).  sharedSecret is from ECDH.
    }

    // SYNC-SERVER-OWN-DATA-JSON
    public static class ServerOwnDataJSON {
        String[] lanAddresses;
    }

}
