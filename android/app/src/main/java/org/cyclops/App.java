package org.cyclops;

import android.app.Application;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.os.Build;

import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;

import com.google.firebase.messaging.RemoteMessage;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class App extends Application {
    // io is a global executor that we use for background IO-related jobs
    public static final ExecutorService io = Executors.newSingleThreadExecutor();

    public static App global;

    public void onCreate() {
        global = this;
        super.onCreate();
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel c = new NotificationChannel("alerts", "Camera alerts", NotificationManager.IMPORTANCE_HIGH);
            getSystemService(NotificationManager.class).createNotificationChannel(c);
        }
    }

    public void showNotification(RemoteMessage.Notification n) {
        NotificationCompat.Builder b = new NotificationCompat.Builder(this, "alerts")
                .setSmallIcon(R.drawable.ic_launcher_foreground)
                .setContentTitle(n.getTitle())
                .setContentText(n.getBody())
                .setAutoCancel(true)
                .setCategory(NotificationCompat.CATEGORY_ALARM);
                //.setPriority(NotificationCompat.PRIORITY_HIGH); // deprecated since API 26

        NotificationManagerCompat.from(this).notify((int) System.currentTimeMillis(), b.build());
    }
}
