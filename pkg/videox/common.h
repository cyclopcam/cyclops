#ifndef _VIDEOX_COMMON_H
#define _VIDEOX_COMMON_H

#ifdef __cplusplus
extern "C" {
#endif

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>
#include <libavutil/imgutils.h>
#include <libavutil/pixfmt.h>
#include <libavutil/opt.h>
#include <libswscale/swscale.h>

typedef struct NALU {
	const void* Data;
	size_t      Size;
} NALU;

char* GetAvErrorStr(int averr);

#ifdef __cplusplus
}
#endif

// C++ internal functions (not exposed to Go)
#ifdef __cplusplus
#include <vector>

enum class MyCodec {
	None,
	H264,
	H265,
};
MyCodec GetMyCodec(AVCodecID codecId);

void FindNALUsAnnexB(const void* packet, size_t packetSize, std::vector<NALU>& nalus);
bool FindNALUsAvcc(const void* packet, size_t packetSize, std::vector<NALU>& nalus);
#endif

#endif // _VIDEOX_COMMON_H