package org.cyclops;

public class NativeBridge {
    static {
        System.loadLibrary("cynative");
    }

    public static native String helloNative();

   // Convert from YUV to RGBA
    public static native void convertYUVToRGBA(int srcType, byte[] yuvData, int width, int height, byte[] rgbaOut);	

}