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

/*
enum ChromaSampling {
	ChromaSampling_420,
	ChromaSampling_422,
	ChromaSampling_444,
} ChromaSampling;

// Planar YUV 420 image
struct YUVImage {
	enum ChromaSampling Chroma;
	int32_t             Width;
	int32_t             Height;
	int32_t             YStride;
	int32_t             UStride;
	int32_t             VStride;
	const void*         Y;
	const void*         U;
	const void*         V;
};
typedef struct YUVImage YUVImage;
*/

char* GetAvErrorStr(int averr);

#ifdef __cplusplus
}
#endif
