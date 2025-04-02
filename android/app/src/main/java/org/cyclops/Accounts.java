package org.cyclops;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.util.Log;
import androidx.browser.customtabs.CustomTabsIntent;

//import com.google.android.gms.auth.api.signin.GoogleSignIn;
//import com.google.android.gms.auth.api.signin.GoogleSignInClient;
//import com.google.android.gms.auth.api.signin.GoogleSignInOptions;
//import okhttp3.OkHttpClient;

//import android.content.pm.PackageInfo;
//import android.content.pm.PackageManager;
//import android.content.pm.Signature;
//import java.security.MessageDigest;

// Accounts helps with authenticating to accounts.cyclopcam.org, so that we can enable
// the use of assisted features such as the proxy/vpn system.
public class Accounts {

    private static final String TAG = "cyclops";

    //public static final int RC_SIGN_IN = 9001;
    //private GoogleSignInClient mGoogleSignInClient;
    //private OkHttpClient httpClient;

    // NOTE. This is not used - I couldn't get anything besides a "10" response from Google auth.
    // So we use signinWeb instead.
    /*
    public void signinNative(Activity activity) {
        Log.d(TAG, "Application ID: " + activity.getPackageName());
        Log.d(TAG, "Getting SHA1 fingerprint");
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            Log.d(TAG, "Getting SHA1 fingerprint FOIR REAL");
            try {
                PackageInfo info = activity.getPackageManager().getPackageInfo(activity.getPackageName(), PackageManager.GET_SIGNING_CERTIFICATES);
                for (Signature sig : info.signingInfo.getApkContentsSigners()) {
                    MessageDigest md = MessageDigest.getInstance("SHA1");
                    md.update(sig.toByteArray());
                    String sha1 = bytesToHex(md.digest());
                    Log.d(TAG, "SHA1 Fingerprint: " + sha1);
                }
            } catch (Exception e) {
                Log.e(TAG, "Error getting SHA1", e);
            }
        }


        // Configure Google Sign-In
        GoogleSignInOptions gso = new GoogleSignInOptions.Builder(GoogleSignInOptions.DEFAULT_SIGN_IN)
                .requestIdToken("25573550938-3q18qodtnlbftfud12bijsf1fjn0vh6t.apps.googleusercontent.com")
                //.requestEmail()
                .build();

        mGoogleSignInClient = GoogleSignIn.getClient(activity, gso);
        httpClient = new OkHttpClient();

        Intent signInIntent = mGoogleSignInClient.getSignInIntent();
        activity.startActivityForResult(signInIntent, RC_SIGN_IN);
    }
    */

    public void signinWeb(Activity activity) {
        // Launch Chrome Custom Tabs to your OAuth sign-in page
        String url = "https://accounts.cyclopcam.org/login.html?return_to=cyclops://auth";
        CustomTabsIntent.Builder builder = new CustomTabsIntent.Builder();
        builder.setShowTitle(false);
        builder.setShareState(CustomTabsIntent.SHARE_STATE_OFF);
        CustomTabsIntent customTabsIntent = builder.build();
        customTabsIntent.launchUrl(activity, Uri.parse(url));
    }

    private String bytesToHex(byte[] bytes) {
        StringBuilder result = new StringBuilder();
        for (byte b : bytes) {
            result.append(String.format("%02X:", b));
        }
        return result.substring(0, result.length() - 1); // Remove trailing colon
    }

}
