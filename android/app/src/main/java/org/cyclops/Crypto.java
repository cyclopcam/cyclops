package org.cyclops;

import android.util.Base64;

import com.google.crypto.tink.subtle.X25519;

import java.security.InvalidKeyException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;

import javax.crypto.Mac;

public class Crypto {
    // Own own temporary keypair.
    // This is the simplest way I can think of to allow the server to sign
    // a challenge. We could also allow the user to create an ed25519 key
    // from it's x25519 key, but I don't understand that mechanism well
    // enough to trust myself with it, so we just do the dumb thing here,
    // and use the Diffie Hellman shared secret as the HMAC key.
    // In order to do Diffie Hellman, we need to create a key for ourselves,
    // which is what we're doing here.
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
    byte[] createChallenge() {
        byte[] challenge = new byte[32];
        SecureRandom random = new SecureRandom();
        random.nextBytes(challenge);
        return challenge;
    }

    // Verify challenge that we've issued to a cyclops server, to prove that it owns
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
}
