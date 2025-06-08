package org.cyclops;

import android.util.Log;
import android.webkit.CookieManager;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;

import okhttp3.Cookie;
import okhttp3.CookieJar;
import okhttp3.HttpUrl;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.WebSocket;
import okhttp3.WebSocketListener;
import okio.ByteString;

/**
 * Streams a binary HEVC (or H264) WebSocket feed into MediaCodec and
 * exposes decoded RGBA frames to callers.
 *
 *  ┌─────────┐ ws.binaryType='arraybuffer'
 *  │   JS    │────────────────┐
 *  └─────────┘                │
 *             (wsUrl)         ▼
 *  ┌─────────────────────────────────┐
 *  │         WebsocketPlayer        │
 *  │  ▸ parse header                │
 *  │  ▸ decoder.sendPacket(video)   │
 *  │  ▸ drain decoder → frameQueue  │
 *  └─────────────────────────────────┘
 */
public class WebsocketPlayer {
    private static final String TAG = "WebsocketPlayer";

    /** One–in ⁄ one-out RGBA frames, latest winners first. */
    private final BlockingQueue<byte[]> frameQueue = new LinkedBlockingQueue<>(8);

    private final VideoDecoder decoder;
    private final WebSocket    ws;
    private final ExecutorService decoderPump;

    /* --------------------------------------------------------------------- */

    public WebsocketPlayer(String wsUrl, String codec, int width, int height) throws Exception {
        String codecMime = "";
        switch (codec) {
            // SYNC-INTERNAL-CODEC-NAMES
            case "h264":
                codecMime = "avc";
                break;
            case "h265":
                codecMime = "hevc";
                break;
        }

        decoder = new VideoDecoder("video/" + codecMime, width, height);

        OkHttpClient client = new OkHttpClient.Builder()
                .cookieJar(new WebViewCookieJar())
                .readTimeout(0, TimeUnit.MILLISECONDS)   // streaming → no read time-out
                .build();

        ws = client.newWebSocket(
                new Request.Builder().url(wsUrl).build(),
                new Listener());

        /*
         * A single background thread that calls decoder.receiveFrame()
         * in a tight loop and drops the frame into frameQueue.
         */
        decoderPump = Executors.newSingleThreadExecutor(r -> new Thread(r, "decoder-pump"));
        decoderPump.execute(this::pumpFrames);
    }


    /* --------------------------------------------------------------------- */
    /*  Public API                                                           */
    /* --------------------------------------------------------------------- */

    /** Returns the next decoded RGBA frame, or {@code null} if none ready. */
    public byte[] pollFrame() {
        return frameQueue.poll();
    }

    /** Call when you are done with the player to release resources. */
    public void close() {
        ws.close(1000, "bye");
        decoderPump.shutdownNow();
        decoder.close();
    }

    /* --------------------------------------------------------------------- */
    /*  Internal                                                            */
    /* --------------------------------------------------------------------- */

    /** WebSocketListener that feeds raw NALUs into the decoder. */
    private final class Listener extends WebSocketListener {

        @Override public void onMessage(WebSocket ws, ByteString bytes) {
            try {
                feedPacket(bytes);
            } catch (Throwable t) {
                t.printStackTrace();
            }
        }

        @Override public void onFailure(WebSocket ws, Throwable t, okhttp3.Response r) {
            t.printStackTrace();
        }
    }

    /**
     * Parse your 16-byte header and push the video payload to {@link VideoDecoder}.
     * Format (little-endian unless noted):
     *
     *  0-3  ↓ headerSize
     *  4-7  ↑ codec32 (big-endian “H264” / “H265”)
     *  8-11 ↓ flags
     * 12-15 ↓ recvId
     * <headerSize…> video
     */
    private void feedPacket(ByteString bs) throws Exception {

        ByteBuffer buf = bs.asByteBuffer();

        // headerSize (LE)
        buf.order(ByteOrder.LITTLE_ENDIAN);
        if (buf.remaining() < 4) return;          // malformed
        int headerSize = buf.getInt();

        // codec32 (BE)
        buf.order(ByteOrder.BIG_ENDIAN);
        int codec32 = buf.getInt();

        // flags + recvId (LE again)
        buf.order(ByteOrder.LITTLE_ENDIAN);
        int flags  = buf.getInt();
        int recvId = buf.getInt();

        // TODO: use flags/recvId if you need key-frame or backlog logic

        /* -------------------------------------- video payload */
        int payloadOffset = headerSize;
        if (payloadOffset > bs.size()) return;    // sanity
        //byte[] video = new byte[bs.size() - payloadOffset];
        //bs.copy(payloadOffset, video, 0, video.length);

        // Fast: points at the same backing array, no copy yet
        ByteString payload = bs.substring(headerSize);

        // MediaCodec needs a byte[], so materialise it once
        decoder.sendPacket(payload.toByteArray());
    }

    /** Continuously pull decoded frames and queue them for consumers. */
    private void pumpFrames() {
        try {
            while (!Thread.currentThread().isInterrupted()) {
                byte[] frame = decoder.receiveFrame();   // may return null
                if (frame != null) {
                    //Log.i(TAG, "pumpFrames: got frame. length = " + frame.length);
                    // Drop the oldest frame if UI is too slow – keeps lag bounded
                    while (!frameQueue.offer(frame)) {
                        frameQueue.poll();
                    }
                } else {
                    Thread.sleep(2);  // tiny back-off to avoid busy wait
                }
            }
        } catch (Throwable t) {
            t.printStackTrace();
        }
    }
}
