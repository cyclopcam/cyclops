package org.cyclops;

import android.graphics.Bitmap;
import android.util.Log;
import android.webkit.WebResourceResponse;

import java.util.HashMap;

public interface Main {
    void webViewBackFailed();
    void navigateToScannedLocalServer(String publicKey, String path, HashMap<String,String> queryParams);
    void setLocalWebviewVisibility(String mode);
    void onLogin(String bearerToken, String sessionCookie);
    void onPostLogin();
    void serverDeleted(String publicKey);
    void switchToServer(String publicKey, boolean allowPreserveUrl);
    boolean isLoggingIn();
    int getContentHeight();
    Bitmap getRemoteViewScreenGrab();
    void clearRemoteViewScreenGrab();
    void createRemoteViewScreenGrab();
    void restartingServer(String publicKey);
    void onNetworkDown(String errorMsg);
    void requestOAuthLogin(String purpose, String provider);
}
