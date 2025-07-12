package org.cyclops;

import android.app.Application;
import android.util.Log;
import androidx.annotation.NonNull;

import com.google.firebase.messaging.FirebaseMessaging;
import com.google.firebase.messaging.RemoteMessage;

import java.util.Map;

public final class FCMReceiverService extends com.google.firebase.messaging.FirebaseMessagingService {

        @Override
        public void onNewToken(@NonNull String token) {
            Log.i("FCM", "onNewToken: " + token);
            setFcmToken(token);
        }

        @Override
        public void onMessageReceived(@NonNull RemoteMessage msg) {
            Log.i("FCM", "Message received");

            State.Notification own = new State.Notification();

            if (msg.getData().size() > 0) {
                Log.i("FCM", "Message has data payload");
                Map<String, String> data = msg.getData();
                for (String key : data.keySet()) {
                    String value = data.get(key);
                    Log.i("FCM", "Key: " + key + " Value: " + value);
                    switch (key) {
                        case "serverPublicKey":
                            own.serverPublicKey = value;
                            break;
                        case "idOnServer":
                            own.idOnServer = Long.parseLong(value);
                            break;
                        case "eventType":
                            own.eventType = value;
                            break;
                        case "time":
                            own.originalTime = Long.parseLong(value);
                            break;
                    }
                }
            }

            if (msg.getNotification() != null) {
                RemoteMessage.Notification n = msg.getNotification();
                own.title = n.getTitle();
                own.body = n.getBody();
                Log.i("FCM", "Message has notification: " + own.title + " -> " + own.body);
            }

            State.global.saveNewNotification(own);

            if (!own.title.equals("")) {
                App.global.showNotification(own);
            }

            // debug code
            //Log.i("FCM", "Saved notification id = " + own.ownId);
            // test retrieving it
            //State.Notification n2 = State.global.getNotification(own.ownId);
            //Log.i("FCM", "Roundtrip: " + n2.serverPublicKey + " ... " + n2.title + " ... " + n2.ownId);
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
