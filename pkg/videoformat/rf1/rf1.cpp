#include "rf1.h"

static_assert(sizeof(CommonIndexHeader) == 32, "IndexHeader size mismatch");
static_assert(sizeof(AudioIndexHeader) == 32, "IndexHeader size mismatch");
static_assert(sizeof(VideoIndexHeader) == 32, "IndexHeader size mismatch");

extern "C" {

//void WriteIndexHeader(const IndexHeader* header, char* data, size_t len) {
//}
//
//void ReadIndexHeader(const char* data, size_t len, IndexHeader* header) {
//}
}
