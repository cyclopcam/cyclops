package org.cyclops;

public class NativeBridge {
    static {
        System.loadLibrary("your_native_lib");
    }

    public static native String helloNative();

   // Convert from YUV to RGBA
    public static native void convertYUVToRGBA(int srcType, byte[] yuvData, int width, int height, byte[] rgbaOut);	

}