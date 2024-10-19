#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>

#ifdef __cplusplus
extern "C" {
#endif

struct NALU {
	const void* Data;
	size_t      Size;
};

char* GetAvErrorStr(int averr);

#define RETURN_ERROR_STATIC(msg)   \
	{                       \
		*err = strdup(msg); \
		return nullptr;     \
	}

#define RETURN_ERROR_STR(msg)             \
	{                               \
		*err = strdup(msg.c_str()); \
		return nullptr;             \
	}

#ifdef __cplusplus
}
#endif
