package org.cyclops;

import android.graphics.Bitmap;
import android.util.Log;
import android.webkit.WebResourceResponse;

public interface Main {
    void webViewBackFailed();
    void navigateToScannedLocalServer(String publicKey);
    void setLocalWebviewVisibility(String mode);
    //WebResourceResponse login(String username, String password);
    void onLogin(String bearerToken, String sessionCookie);
    void switchToServer(String publicKey);
    int getContentHeight();
    //void notifyRegisteredServersChanged();
    Bitmap getRemoteViewScreenGrab();
    void clearRemoteViewScreenGrab();
    void createRemoteViewScreenGrab();
    void onNetworkDown(String errorMsg);
}
