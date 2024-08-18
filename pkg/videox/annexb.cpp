#include <stdint.h>
#include <string.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
From this great post:
https://stackoverflow.com/questions/24884827/possible-locations-for-sequence-picture-parameter-sets-for-h-264-stream/24890903#24890903

	Start codes work because the four byte sequences 00.00.00, 00.00.01, 00.00.02
	and 00.00.03 are illegal within a non-RBSP NALU. So when creating a NALU, care
	is taken to escape these values that could otherwise be confused with a start
	code. This is accomplished by inserting an ‘Emulation Prevention’ byte 03,
	so that 00.00.01 becomes 00.00.03.01

Return the number of bytes written if successful, or 0 if there was not enough space.
*/
size_t EncodeAnnexB(const void* src, size_t srcLen, void* dst, size_t dstLen) {
	const uint8_t* in     = (const uint8_t*) src;
	uint8_t*       out    = (uint8_t*) dst;
	uint8_t*       outEnd = out + dstLen;

	if (dstLen < srcLen || srcLen == 0)
		return 0;

	if (srcLen < 3) {
		for (size_t i = 0; i < srcLen; i++) {
			out[i] = in[i];
		}
		return srcLen;
	}

	// example byte stream:
	// 0  1  2  3
	// 00 00 01 F9

	int sum = (int) in[0] + (int) in[1];
	out[0]  = in[0];
	out[1]  = in[1];
	out += 2;

	// Don't emit a 0x03 more than once every 2 bytes.
	// This comes into play with a string of zeros.
	intmax_t tick = 0;

	for (size_t i = 2; i < srcLen; i++) {
		sum += (int) in[i];

		if (sum <= 3 && in[i - 2] == 0 && in[i - 1] == 0 && tick >= 0) {
			// We only need to check for buffer space at the start, and when we encounter an escape.
			// The naive thing to do would be to check for buffer space on every loop iteration.
			// This check is more expensive, but escaped bytes are very rare.
			if (srcLen - i + 1 > outEnd - out)
				return 0;
			*out++ = 3;
			*out++ = in[i];
			tick   = -2;
		} else {
			*out++ = in[i];
		}
		sum -= (int) in[i - 2];
		tick++;
	}

	return out - (uint8_t*) dst;
}

// For testing, does not do any encoding, but simply a memcpy
size_t EncodeAnnexB_Null(const void* src, size_t srcLen, void* dst, size_t dstLen) {
	if (dstLen < srcLen || srcLen == 0)
		return 0;
	memcpy(dst, src, srcLen);
	return srcLen;
}

// This is copied from the ffmpeg source code, so we use it to verify our implementation.
size_t EncodeAnnexB_Ref(const void* src, size_t srcLen, void* dst, size_t dstLen) {
	const uint8_t* in  = (const uint8_t*) src;
	uint8_t*       out = (uint8_t*) dst;

	// You must allocate a pessimistic buffer size.
	if (dstLen < srcLen * 3 / 2)
		return 0;

	size_t zeroRun = 0;
	size_t j       = 0;
	for (size_t i = 0; i < srcLen; i++) {
		if (zeroRun < 2) {
			if (in[i] == 0)
				zeroRun++;
			else
				zeroRun = 0;
		} else {
			if ((in[i] & ~3) == 0) {
				// emulation_prevention_three_byte
				out[j++] = 3;
			}
			zeroRun = in[i] == 0;
		}
		out[j++] = in[i];
	}

	return j;
}

// Return the number of bytes written if successful, or 0 if there was not enough space.
size_t DecodeAnnexB(const void* src, size_t srcLen, void* dst, size_t dstLen) {
	const uint8_t* in  = (const uint8_t*) src;
	uint8_t*       out = (uint8_t*) dst;
	size_t         i   = 0;
	size_t         j   = 0;

	// Just allocate a buffer the same size as the input. This way we don't
	// need to check the output buffer size when transcoding.
	if (dstLen < srcLen || srcLen == 0)
		return 0;

	for (; i < srcLen && i < 2; i++) {
		out[j++] = in[i];
	}

	// example byte stream:
	// 00 00 03 01 F9  ->  00 00 01 F9
	// 00 00 03 00 00 03 -> 00 00 00 00 03

	for (; i < srcLen; i++) {
		if (in[i] == 3 && in[i - 2] == 0 && in[i - 1] == 0) {
			// skip emulation_prevention_three_byte
		} else {
			out[j++] = in[i];
		}
	}

	return j;
}

// This is copied from the ffmpeg source code, so we use it to verify our implementation.
size_t DecodeAnnexB_Ref(const void* src, size_t srcLen, void* dst, size_t dstLen) {
	const uint8_t* in  = (const uint8_t*) src;
	uint8_t*       out = (uint8_t*) dst;
	size_t         i   = 0;
	size_t         j   = 0;

	if (dstLen < srcLen)
		return 0;

	while (i + 2 < srcLen)
		if (!in[i] && !in[i + 1] && in[i + 2] == 3) {
			out[j++] = in[i++];
			out[j++] = in[i++];
			i++; // remove emulation_prevention_three_byte
		} else
			out[j++] = in[i++];

	while (i < srcLen)
		out[j++] = in[i++];

	return j;
}

#ifdef __cplusplus
}
#endif
