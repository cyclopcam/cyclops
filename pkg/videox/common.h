#ifdef __cplusplus
extern "C" {
#endif

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>
#include <libavutil/imgutils.h>
#include <libavutil/pixfmt.h>
#include <libswscale/swscale.h>

typedef struct NALU {
	const void* Data;
	size_t      Size;
} NALU;

char* GetAvErrorStr(int averr);

#ifdef __cplusplus
}
#endif
