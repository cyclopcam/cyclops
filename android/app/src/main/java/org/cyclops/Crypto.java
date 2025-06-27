package org.cyclops;

import android.util.Base64;

import com.google.crypto.tink.subtle.X25519;

import java.security.InvalidKeyException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;

import javax.crypto.Mac;

public class Crypto {
    // Our own temporary keypair.
    // This is the simplest way I can think of to allow the server to sign
    // a challenge. We could also allow the user to create an ed25519 key
    // from it's x25519 key, but I don't understand that mechanism well
    // enough to trust myself with it, so we just do the dumb thing here,
    // and use the Diffie Hellman shared secret as the HMAC key.
    // In order to do Diffie Hellman, we need to create a key for ourselves,
    // which is what we're doing here.
    // Note that https://github.com/teslamotors/liblithium is a proof of concept
    // where they sign using an X25519 curve, but I'm sticking with this
    // for now, because I understand it.
    byte[] ownPrivateKey;
    byte[] ownPublicKey;

    Crypto() {
        // create our own temporary x25519 keypair
        try {
            ownPrivateKey = X25519.generatePrivateKey();
            ownPublicKey = X25519.publicFromPrivate(ownPrivateKey);
        } catch (InvalidKeyException e) {
        }
    }

    // create a 32 byte challenge message, containing random bytes
    static byte[] createChallenge() {
        byte[] challenge = new byte[32];
        SecureRandom random = new SecureRandom();
        random.nextBytes(challenge);
        return challenge;
    }

    // Create a random 21 byte string to identify this device to accounts.cyclopcam.org.
    static String createDeviceId() {
        byte[] deviceId = new byte[21];
        SecureRandom random = new SecureRandom();
        random.nextBytes(deviceId);
        return Base64.encodeToString(deviceId, Base64.URL_SAFE | Base64.NO_PADDING | Base64.NO_WRAP);
    }

    // Verify the challenge that we've issued to a cyclops server, to prove that it owns
    // the public key that it claims.
    boolean verifyChallenge(String serverPublicKey, byte[] challenge, byte[] response) {
        if (challenge.length != 32 || response.length != 32) {
            return false;
        }
        try {
            byte[] shared = X25519.computeSharedSecret(ownPrivateKey, Base64.decode(serverPublicKey, Base64.DEFAULT));
            // compute HMAC SHA256 of challenge with shared secret
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new javax.crypto.spec.SecretKeySpec(shared, "HmacSHA256"));
            byte[] expected = mac.doFinal(challenge);
            return MessageDigest.isEqual(expected, response);
        } catch (InvalidKeyException | NoSuchAlgorithmException e) {
            return false;
        }
    }

    // Return the shortkey for the given server, which is hex(pubkey[:10])
    static String shortKeyForServer(String serverPublicKey) {
        // Decode from base64 to byte array
        byte[] publicKey = Base64.decode(serverPublicKey, Base64.DEFAULT);
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < 10; i++) {
            sb.append(String.format("%02x", publicKey[i]));
        }
        return sb.toString();
    }
}
