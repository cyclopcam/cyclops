package org.cyclops;

import android.app.Application;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
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

    public void showNotification(State.Notification n) {
        Intent intent = new Intent(this, MainActivity.class);
        intent.addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP); // Clears the activity stack to bring the user directly to this activity
        intent.putExtra("notification", n.ownId);

        // Wrap the Intent in a PendingIntent
        int pendingIntentFlags = PendingIntent.FLAG_UPDATE_CURRENT;
        pendingIntentFlags |= PendingIntent.FLAG_IMMUTABLE; // Required for API 23+ for security, but especially API 31+
        PendingIntent pendingIntent = PendingIntent.getActivity(this, 0 /* Request code */, intent, pendingIntentFlags);

        NotificationCompat.Builder b = new NotificationCompat.Builder(this, "alerts")
                .setSmallIcon(R.drawable.ic_launcher_foreground)
                .setContentTitle(n.title)
                .setContentText(n.body)
                .setAutoCancel(true)
                .setCategory(NotificationCompat.CATEGORY_ALARM)
                .setContentIntent(pendingIntent);
                //.setPriority(NotificationCompat.PRIORITY_HIGH); // deprecated since API 26

        NotificationManagerCompat.from(this).notify(n.androidId(), b.build());
    }
}
