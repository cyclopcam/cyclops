package org.cyclops;

import static android.content.Intent.FLAG_ACTIVITY_NEW_TASK;

import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;
import android.net.Uri;
import android.os.Build;
import android.util.Log;
import androidx.browser.customtabs.CustomTabsIntent;

import com.google.gson.Gson;

import java.security.MessageDigest;
import java.util.HashMap;
import java.util.Map;

import okhttp3.MediaType;

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
    static final Accounts global = new Accounts();

    private static final String TAG = "cyclops";
    private final HttpClient httpClient;

    public static class OauthLinkJSON {
        String provider = ""; // eg "google"
        String id = ""; // ID at the provider (eg Google ID, such as "1234567890")
        String email = "";
        String displayName = "";
    }

    // Response from accounts.cyclopcam.org /api/auth/whoami
    public static class WhoamiJSON {
        String id = ""; // ID with accounts.cyclopcam.org
        String email = "";
        String displayName = "";
        OauthLinkJSON[] oauth;
    }

    public static class CreateTokenJSON {
        String token = "";
        long expiresAt = 0; // Unix seconds (0/omitted if no expiration)
    }

    public static class MobileDeviceRegisterJSON {
        String deviceId = "";
        String fcmToken = "";
        String platform = "";
        String name = "";
    }

    Accounts() {
        httpClient = new HttpClient();
    }

    //public static final int RC_SIGN_IN = 9001;
    //private GoogleSignInClient mGoogleSignInClient;
    //private OkHttpClient httpClient;

    public void debugPrintSigningCert(Activity activity) {
        Log.d(TAG, "Application ID: " + activity.getPackageName());
        Log.d(TAG, "Getting SHA1 fingerprint");
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            Log.d(TAG, "Getting SHA1 fingerprint FOR REAL");
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
    }

    // NOTE. This is not used - I couldn't get anything besides a "10" response from Google auth.
    // So we use signinWeb instead.
    /*
    public void signinNative(Activity activity) {
        debugPrintSigningCert();

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

    public void signinWeb(Activity activity, String provider) {
        Log.i(TAG, "signinWeb for " + provider);
        // v1
        String redirectUri = "cyclops://auth";
        // v2
        //String redirectUri = "https://cyclopcam.org/android-auth";

        // Launch Chrome Custom Tabs to your OAuth sign-in page
        String url = "https://accounts.cyclopcam.org/login?return_to=" + redirectUri;
        if (!provider.equals("")) {
            // I have NO IDEA WHY (logcat logs give me no clue). If we point the custom tab
            // directly at this URL, then the auth completes (I can see it on accounts.cyclopcam.org),
            // but the redirect back to cyclops://auth never happens (our activity is never resumed).
            // So, as a workaround, I point the custom tab to /login?click=google, and the
            // server then waits 200ms, before simulating the click of the "login with google" button.
            url = "https://accounts.cyclopcam.org/api/auth/oauth2/" + provider + "/login?return_to=" + redirectUri;
            // mkay, this workaround didn't fix the problem. sigh.
            //url = "https://accounts.cyclopcam.org/login?click=" + provider + "&return_to=" + redirectUri;
            // It turned out that FLAG_ACTIVITY_NEW_TASK was necessary
            // See all these other people experiencing the same issue:
            // https://stackoverflow.com/questions/36084681/chrome-custom-tabs-redirect-to-android-app-will-close-the-app
        }
        CustomTabsIntent.Builder builder = new CustomTabsIntent.Builder();
        //builder.setShowTitle(false);
        builder.setShareState(CustomTabsIntent.SHARE_STATE_OFF);
        CustomTabsIntent customTabsIntent = builder.build();

        // This next line is crucial to ensure that after the OAuth flow completes,
        // our app doesn't get backgrounded.
        customTabsIntent.intent.setFlags(FLAG_ACTIVITY_NEW_TASK);

        customTabsIntent.launchUrl(activity, Uri.parse(url));
    }

    // Check with accounts.cyclopcam.org if we are signed in with a specific OAuth provider
    public boolean isSignedinWithOAuthProvider(String provider, String token) throws RuntimeException {
        if (token == null || token.equals("")) {
            return false;
        }
        Log.i(TAG, "isSignedinWithOAuthProvider");
        HttpClient.Response response = httpClient.GET("https://accounts.cyclopcam.org/api/auth/whoami", new HashMap<>(Map.of("Authorization", "Bearer " + token)));
        if (response.Error != null) {
            Log.e(TAG, "isSignedinWithOAuthProvider Error: " + response.Error);
            throw new RuntimeException(response.Error);
        }
        Log.i(TAG, "isSignedinWithOAuthProvider: " + response.BodyOrStatusString);
        if (response.Resp.code() != 200) {
            return false;
        }
        WhoamiJSON whoami = new Gson().fromJson(response.Body, WhoamiJSON.class);
        for (OauthLinkJSON oauth : whoami.oauth) {
            if (oauth.provider.equals(provider)) {
                return true;
            }
        }
        return false;
    }

    // Acquire an IdentityToken from accounts.cyclopcam.org. An IdentityToken is a short-lived
    // token (eg 1 minute lifetime) that a cyclops server can use to verify the caller's
    // identity.
    // Returns an empty string on success, or an error otherwise.
    public CreateTokenJSON getIdentityToken(String token) throws RuntimeException {
        Log.i(TAG, "getIdentityToken");
        HttpClient.Response response = httpClient.POST("https://accounts.cyclopcam.org/api/auth/createIdentityToken", new HashMap<>(Map.of("Authorization", "Bearer " + token)));
        if (response.Error != null) {
            Log.e(TAG, "getIdentityToken network failed: " + response.Error);
            throw new RuntimeException(response.Error);
        }
        if (response.Resp.code() != 200) {
            Log.e(TAG, "getIdentityToken failed: " + response.BodyOrStatusString);
            throw new RuntimeException(response.BodyOrStatusString);
        }
        Log.i(TAG, "getIdentityToken success: " + response.Body);
        return new Gson().fromJson(response.Body, CreateTokenJSON.class);
    }

    private String bytesToHex(byte[] bytes) {
        StringBuilder result = new StringBuilder();
        for (byte b : bytes) {
            result.append(String.format("%02X:", b));
        }
        return result.substring(0, result.length() - 1); // Remove trailing colon
    }

    public void sendFcmToken(String fcmToken, String deviceId, String authToken) {
        MobileDeviceRegisterJSON device = new MobileDeviceRegisterJSON();
        device.deviceId = deviceId;
        device.fcmToken = fcmToken;
        device.platform = "android";
        device.name = Build.MANUFACTURER + " " + Build.MODEL;
        Log.i(TAG, "sendFcmToken: " + new Gson().toJson(device));
        HttpClient.Response response = httpClient.POST(
                "https://accounts.cyclopcam.org/api/mobileDevice/register",
                new HashMap<>(Map.of("Authorization", "Bearer " + authToken)),
                MediaType.parse("application/json"),
                new Gson().toJson(device).getBytes());
        if (response.Error != null) {
            Log.e(TAG, "mobileDevice/register network failed: " + response.Error);
        } else if (response.Resp.code() != 200) {
            Log.e(TAG, "mobileDevice/register failed: " + response.BodyOrStatusString);
        } else {
            Log.i(TAG, "mobileDevice/register succeeded");
        }
    }
}
