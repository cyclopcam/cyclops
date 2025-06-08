package org.cyclops;

import android.media.MediaCodec;
import android.media.MediaCodecInfo;
import android.media.MediaFormat;
import android.util.Log;
import java.nio.ByteBuffer;

public class VideoDecoder {
    private static final String TAG = "VideoDecoder";
    //private static final String MIME_TYPE = "video/hevc";
    private static final int TIMEOUT_US = 10000;

    private MediaCodec codec;
    private boolean isConfigured = false;
    private int width;
    private int height;
    private int outputFormat = MediaCodecInfo.CodecCapabilities.COLOR_FormatYUV422PackedSemiPlanar;

    // Image formats:
    // I've only seen 21 in the wild, on my Xiaomi Redmi Note 11 Pro
    // 21 = NV21
    // 19 = MediaCodecInfo.CodecCapabilities.COLOR_FormatYUV420Planar
    // 21 = MediaCodecInfo.CodecCapabilities.COLOR_FormatYUV422PackedSemiPlanar

    public VideoDecoder(String codecMime, int width, int height) throws Exception {
        this.width = width;
        this.height = height;
        MediaFormat format = MediaFormat.createVideoFormat(codecMime, width, height);
        codec = MediaCodec.createDecoderByType(codecMime);
        codec.configure(format, null, null, 0);
        codec.start();
        isConfigured = true;
    }

    public void sendPacket(byte[] packet) throws Exception {
        if (!isConfigured) return;

        int inputBufferId = codec.dequeueInputBuffer(TIMEOUT_US);
        if (inputBufferId >= 0) {
            ByteBuffer inputBuffer = codec.getInputBuffer(inputBufferId);
            inputBuffer.clear();
            inputBuffer.put(packet);

            // Queue the NALU with a timestamp (optional: adjust for live or recorded streams)
            codec.queueInputBuffer(inputBufferId, 0, packet.length, System.nanoTime() / 1000, 0);
        }
    }

    public byte[] receiveFrame() {
        if (!isConfigured) return null;

        MediaCodec.BufferInfo bufferInfo = new MediaCodec.BufferInfo();
        while (true) {
            int outputBufferId = codec.dequeueOutputBuffer(bufferInfo, TIMEOUT_US);

            if (outputBufferId == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED) {
                MediaFormat fmt = codec.getOutputFormat();
                outputFormat = fmt.getInteger(MediaFormat.KEY_COLOR_FORMAT);
                Log.i(TAG, "Output format changed: " + outputFormat);
                // loop again, because there might be a frame ready
            } else if (outputBufferId >= 0) {
                ByteBuffer outputBuffer = codec.getOutputBuffer(outputBufferId);
                byte[] yuvData = new byte[bufferInfo.size];
                outputBuffer.get(yuvData);

                codec.releaseOutputBuffer(outputBufferId, false);

                byte[] rgba = new byte[width * height * 4];
                NativeBridge.convertYUVToRGBA(outputFormat, yuvData, width, height, rgba);
                return rgba;
            } else {
                return null;
            }
        }
    }

    /**
     * Fast integer YUV420P ➜ RGBA converter.
     *
     * @param yuvData  input frame in YUV420P (Y plane, then U plane, then V plane)
     * @param width    frame width  (must be even)
     * @param height   frame height (must be even)
     * @return         RGBA buffer of size width*height*4
     */
    /*
    public static byte[] convertYUV420ToRGBA(byte[] yuvData, int width, int height) {
        // Grok 3
        // Calculate dimensions and offsets
        int uvWidth = width / 2;
        int uvHeight = height / 2;
        int ySize = width * height;
        int uvSize = uvWidth * uvHeight;

        // Allocate output array for RGBA data (4 bytes per pixel)
        byte[] rgba = new byte[width * height * 4];
        int outputIndex = 0;

        // Process each pixel
        for (int y = 0; y < height; y++) {
            int yIndex = y * width;
            int uRow = (y / 2) * uvWidth;
            for (int x = 0; x < width; x++) {
                // Calculate indices for Y, U, and V planes
                int x_u = x / 2;
                int uIndex = ySize + uRow + x_u;
                int vIndex = ySize + uvSize + uRow + x_u;

                // Extract Y, U, V values as unsigned integers (0-255)
                int Y = yuvData[yIndex + x] & 0xFF;
                int U = yuvData[uIndex] & 0xFF;
                int V = yuvData[vIndex] & 0xFF;

                // Adjust U and V for chrominance
                int Cb = U - 128;
                int Cr = V - 128;

                // Convert to RGB using integer arithmetic (approximated coefficients)
                int R = Y + (359 * Cr >> 8);           // 1.402 ≈ 359/256
                int G = Y - ((88 * Cb + 182 * Cr) >> 8); // 0.344 ≈ 88/256, 0.714 ≈ 182/256
                int B = Y + (454 * Cb >> 8);           // 1.772 ≈ 454/256

                // Clamp RGB values to [0, 255]
                R = Math.max(0, Math.min(255, R));
                G = Math.max(0, Math.min(255, G));
                B = Math.max(0, Math.min(255, B));

                // Write RGBA to output array
                rgba[outputIndex++] = (byte) R;
                rgba[outputIndex++] = (byte) G;
                rgba[outputIndex++] = (byte) B;
                rgba[outputIndex++] = (byte) 255;
            }
        }
        return rgba;
        */

        // O3
        /*
        final int frameSize   = width * height;           // Y plane size
        final int quarterSize = frameSize >> 2;           // size of each chroma plane
        final byte[] rgba     = new byte[frameSize * 4];

        int yIndex = 0;                                   // index inside Y plane

        // Process each line
        for (int y = 0; y < height; y++) {
            final int uvRow = (y >> 1) * (width >> 1);    // top-left chroma sample for this row

            for (int x = 0; x < width; x++, yIndex++) {
                // --- fetch Y, U, V ---------------------------------------------------
                //final int Y = (yuvData[yIndex] & 0xFF) - 16;                // nominal range 16-235
                final int Y = yuvData[yIndex];
                final int uvCol  = (x >> 1);                                // 2×2 subsampling
                final int U = (yuvData[frameSize + uvRow + uvCol]       & 0xFF) - 128;
                final int V = (yuvData[frameSize + quarterSize + uvRow + uvCol] & 0xFF) - 128;

                // guard against negative Y after offset
                //final int C = Y < 0 ? 0 : Y;
                final int C = Y;

                // --- integer YUV ➜ RGB (BT.601) --------------------------------------
                // R = 1.164*C + 1.596*E
                // G = 1.164*C - 0.392*D - 0.813*E
                // B = 1.164*C + 2.017*D
                final int R = (298 * C + 409 * V + 128) >> 8;
                final int G = (298 * C - 100 * U - 208 * V + 128) >> 8;
                final int B = (298 * C + 516 * U + 128) >> 8;

                // --- clamp to [0,255] with branch-free math --------------------------
                final int r = (R & (~R >> 31)) | 255 - ((R - 255) & ((R - 255) >> 31));
                final int g = (G & (~G >> 31)) | 255 - ((G - 255) & ((G - 255) >> 31));
                final int b = (B & (~B >> 31)) | 255 - ((B - 255) & ((B - 255) >> 31));

                // --- store RGBA ------------------------------------------------------
                final int out = yIndex << 2;     // *4
                rgba[out    ] = (byte) r;
                rgba[out + 1] = (byte) g;
                rgba[out + 2] = (byte) b;
                rgba[out + 3] = (byte) 0xFF;     // opaque alpha
            }
        }
        return rgba;
    }
         */

    public void close() {
        if (codec != null) {
            codec.stop();
            codec.release();
            codec = null;
        }
    }
}
