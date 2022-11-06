package org.cyclops;

import android.graphics.Bitmap;
import android.util.Log;

public interface Main {
    void webViewBackFailed();
    void navigateToScannedLocalServer(String publicKey);
    void switchToServerByPublicKey(String publicKey);
    void setLocalWebviewVisibility(String mode);
    void onLogin(String bearerToken);
    int getContentHeight();
    //void notifyRegisteredServersChanged();
    Bitmap getRemoteViewScreenGrab();
    void clearRemoteViewScreenGrab();
    void createRemoteViewScreenGrab();
}
