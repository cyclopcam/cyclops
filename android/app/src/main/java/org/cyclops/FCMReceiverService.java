package org.cyclops;

import android.app.Application;
import android.util.Log;
import androidx.annotation.NonNull;

import com.google.firebase.messaging.FirebaseMessaging;
import com.google.firebase.messaging.RemoteMessage;

public final class FCMReceiverService extends com.google.firebase.messaging.FirebaseMessagingService {

        @Override
        public void onNewToken(@NonNull String token) {
            Log.i("FCM", "onNewToken: " + token);
            setFcmToken(token);
        }

        @Override
        public void onMessageReceived(@NonNull RemoteMessage msg) {
            // 1. Handle data-only messages yourself
            if (msg.getData().size() > 0) {
                Log.i("FCM", "Message with data payload received");
                //handleDataPayload(msg.getData());
            }

            // 2. If the message also carried a notification and the app is
            //    in the foreground, build & show it manually.
            if (msg.getNotification() != null) {
                Log.i("FCM", "Message with notification received");
                App.global.showNotification(msg.getNotification());
            }
        }

        public static void refreshToken() {
            FirebaseMessaging.getInstance().getToken().addOnCompleteListener(task -> {
                if (!task.isSuccessful()) {
                    Log.w("FCM", "token fetch failed", task.getException());
                    return;
                }
                String token = task.getResult();
                setFcmToken(token);
            });
        }

        private static void setFcmToken(String fcmToken) {
            Log.i("FCM", "New token: " + fcmToken);
            State.global.setFcmToken(fcmToken);
            sendFcmTokenToCloud();
        }

        public static void sendFcmTokenToCloud() {
            App.io.execute(() -> {
                String fcmToken = State.global.getFcmToken();
                String accountsToken = State.global.getAccountsToken();
                if (fcmToken.equals("")) {
                    Log.i("FCM", "No FCM token yet, so can't send FCM token");
                    return;
                }
                if (accountsToken.equals("")) {
                    Log.i("FCM", "No accounts token yet, so can't send FCM token");
                    return;
                }
                Accounts.global.sendFcmToken(fcmToken, State.global.getDeviceId(), State.global.getAccountsToken());
            });
        }


    }
