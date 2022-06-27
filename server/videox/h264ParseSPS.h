#include <stdint.h>
#include <string.h>

#ifdef __cplusplus
extern "C" {
#endif

void ParseSPS(const void* buf, size_t len, int* width, int* height);

#ifdef __cplusplus
}
#endif