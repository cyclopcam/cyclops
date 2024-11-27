#include "pagealloc.h"

#include <atomic>
#include <unistd.h>
//#include <stdio.h>

std::atomic<size_t> PAGE_SIZE{0};

#ifdef __cplusplus
extern "C" {
#endif

// Allocate page-aligned memory
void* PageAlignedAlloc(size_t size) {
	// Get the system page size
	size_t pageSize = PAGE_SIZE.load(std::memory_order_acquire);
	if (pageSize == 0) {
		// This sysconf call is 100ns on Rpi5, which is why we cache it
		pageSize = sysconf(_SC_PAGESIZE);
		if (pageSize == (size_t) -1) {
			return nullptr;
		}
		PAGE_SIZE.store(pageSize);
	}

	// Round up the size to the nearest page size
	size = (size + pageSize - 1) & ~(pageSize - 1);

	//fprintf(stderr, "Allocating %d\n", (int) size);

	void* ptr = nullptr;
	int   ret = posix_memalign(&ptr, pageSize, size);
	if (ret != 0) {
		return nullptr;
	}

	return ptr;
}

void PageAlignedFree(void* ptr) {
	free(ptr);
}

#ifdef __cplusplus
}
#endif
