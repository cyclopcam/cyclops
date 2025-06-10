package org.cyclops;

import android.media.MediaCodec;
import android.media.MediaCodecInfo;
import android.media.MediaFormat;
import android.util.Log;
import java.nio.ByteBuffer;

public class VideoDecoder {
    private static final String TAG = "VideoDecoder";
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

    public void close() {
        if (codec != null) {
            codec.stop();
            codec.release();
            codec = null;
        }
    }
}
