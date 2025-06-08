#include <jni.h>
#include <libyuv.h>
#include <android/log.h>
#include <string>

#define TAG "NativeBridge"
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, TAG, __VA_ARGS__)

static int LastSrcType = 0;

extern "C" JNIEXPORT jstring JNICALL
Java_org_cyclops_NativeBridge_helloNative(JNIEnv* env, jobject self) {
	return env->NewStringUTF("Hello from native C++");
}

extern "C" JNIEXPORT void JNICALL
Java_org_cyclops_NativeBridge_convertYUVToRGBA(JNIEnv* env, jclass clazz,
                                               int        srcType,
                                               jbyteArray yuvData,
                                               jint width, jint height,
                                               jbyteArray rgbaOut) {
	int      res      = 0;
	jbyte*   yuv_ptr  = env->GetByteArrayElements(yuvData, nullptr);
	jbyte*   rgba_ptr = env->GetByteArrayElements(rgbaOut, nullptr);
	uint8_t* dst_rgba = reinterpret_cast<uint8_t*>(rgba_ptr);

	// kYvuJPEGConstants: YUV are full 0..255 range
	// kYvuI601Constants: Limited 16..235 range for Y, 16..240 for UV

	// I'm using kYvuI601Constants for now, because that's what my cameras emit,
	// but we should really be reading this from the codec.

	// Also, note that there are two flavours of these constants:
	// 1. kYuv
	// 2. kYvu
	// We can conveniently use these to flip between RGBA and BGRA.

	auto kMatrix = &libyuv::kYvuI601Constants;

	if (srcType == 19) {
		int y_size  = width * height;
		int uv_size = (width / 2) * (height / 2);

		const uint8_t* src_y = reinterpret_cast<const uint8_t*>(yuv_ptr);
		const uint8_t* src_u = src_y + y_size;
		const uint8_t* src_v = src_u + uv_size;

		int res = libyuv::I420ToARGBMatrix(
		    src_y, width,
		    src_u, width / 2,
		    src_v, width / 2,
		    dst_rgba, width * 4,
		    kMatrix,
		    width, height);
	} else if (srcType == 21) {
		int y_size = width * height;

		const uint8_t* src_y  = reinterpret_cast<const uint8_t*>(yuv_ptr);
		const uint8_t* src_vu = src_y + y_size; // In NV21, VU interleaved follows Y

		// full range (0-255) YUV to RGBA conversion
		auto kMatrix = &libyuv::kYvuI601Constants;

		int res = libyuv::NV21ToARGBMatrix(
		    src_y, width,
		    src_vu, width,
		    dst_rgba, width * 4,
		    kMatrix,
		    width, height);
	} else {
		if (srcType != LastSrcType) {
			LOGE("Unsupported YUV format %d", srcType);
			LastSrcType = srcType;
		}
	}

	if (res != 0) {
		LOGE("libyuv::Transcode failed with error code %d", res);
	}

	env->ReleaseByteArrayElements(yuvData, yuv_ptr, JNI_ABORT);
	env->ReleaseByteArrayElements(rgbaOut, reinterpret_cast<jbyte*>(dst_rgba), 0);
}

/*
extern "C" JNIEXPORT void JNICALL
Java_org_cyclops_NativeBridge_convertNV21ToRGBA(JNIEnv* env, jclass clazz,
                                                jbyteArray yuvData,
                                                jint width, jint height,
                                                jbyteArray rgbaOut) {
	int y_size = width * height;

	jbyte* yuv_ptr  = env->GetByteArrayElements(yuvData, nullptr);
	jbyte* rgba_ptr = env->GetByteArrayElements(rgbaOut, nullptr);

	const uint8_t* src_y    = reinterpret_cast<const uint8_t*>(yuv_ptr);
	const uint8_t* src_vu   = src_y + y_size; // In NV21, VU interleaved follows Y
	uint8_t*       dst_rgba = reinterpret_cast<uint8_t*>(rgba_ptr);

	// full range (0-255) YUV to RGBA conversion
	auto kMatrix = &libyuv::kYvuI601Constants;

	int res = libyuv::NV21ToARGBMatrix(
	    src_y, width,
	    src_vu, width,
	    dst_rgba, width * 4,
	    kMatrix,
	    width, height);

	if (res != 0) {
		LOGE("libyuv::NV21ToARGBMatrix failed with error code %d", res);
	}

	env->ReleaseByteArrayElements(yuvData, yuv_ptr, JNI_ABORT);
	env->ReleaseByteArrayElements(rgbaOut, reinterpret_cast<jbyte*>(dst_rgba), 0);
}
*/