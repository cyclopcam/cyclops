#include <stdint.h>
#include <string.h>

#ifdef __cplusplus
extern "C" {
#endif

void ParseH264SPS(const void* buf, size_t len, int* width, int* height);
void ParseH265SPS(const void* buf, size_t len, int* width, int* height);

#ifdef __cplusplus
}
#endif