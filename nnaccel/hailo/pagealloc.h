#pragma once

#include <stdint.h>
#include <stdlib.h>

#ifdef __cplusplus
extern "C" {
#endif

void* PageAlignedAlloc(size_t bytes);
void  PageAlignedFree(void* ptr);

#ifdef __cplusplus
}
#endif
