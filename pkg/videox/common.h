#ifdef __cplusplus
extern "C" {
#endif

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>
#include <libavutil/imgutils.h>
#include <libswscale/swscale.h>

struct NALU {
	const void* Data;
	size_t      Size;
};

char* GetAvErrorStr(int averr);

#ifdef __cplusplus
}
#endif
