#include <stdlib.h>
#include <stdint.h>

// AudioIndexHeader and VideoIndexHeader share their first 24 bytes (at least in name and element size),
// but we can't use unions in here, because they're inaccessible from Go (Cgo) code.
// As a workaround, we have a CommonIndexHeader struct, which we use like a union from Go code.

// Common index header, used by both Video and Audio.
typedef struct _CommonIndexHeader {
	char     Magic[4];
	char     Codec[4];
	uint32_t Flags;
	uint32_t CodecFlags;
	uint64_t TimeBase;
	uint8_t  Other[8];
} CommonIndexHeader;

// Header of the index file for one track (audio or video)
typedef struct _AudioIndexHeader {
	char     Magic[4];
	char     Codec[4];
	uint32_t Flags;
	uint32_t CodecFlags;
	uint64_t TimeBase;
	uint8_t  Reserved[8];
} AudioIndexHeader;

typedef struct _VideoIndexHeader {
	char     Magic[4];
	char     Codec[4];
	uint32_t Flags;
	uint32_t CodecFlags;
	uint64_t TimeBase;
	uint16_t Width;
	uint16_t Height;
	uint8_t  Reserved[4];
} VideoIndexHeader;

#ifdef __cplusplus
extern "C" {
#endif

//void WriteIndexHeader(const IndexHeader* header, char* data, size_t len);
//void ReadIndexHeader(const char* data, size_t len, IndexHeader* header);

#ifdef __cplusplus
}
#endif
