#pragma once

#include <stdint.h>
#include <stdlib.h>

void* PageAlignedAlloc(size_t bytes);
void  PageAlignedFree(void* ptr);