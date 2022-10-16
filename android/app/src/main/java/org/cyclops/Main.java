package org.cyclops;

import android.graphics.Bitmap;
import android.util.Log;

public interface Main {
    void webViewBackFailed();
    void navigateToServer(String url, boolean addToHistory, State.Server server);
    void switchToServerByPublicKey(String publicKey);
    void showMenu(String mode);
    int getContentHeight();
    Bitmap getRemoteViewScreenGrab();
    void createRemoteViewScreenGrab();
}
