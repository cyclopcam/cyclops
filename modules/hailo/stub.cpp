// This is a stub that builds a static library with the same
// exports as our actual Hailo wrapper library. This makes it
// easy from the Go side, so that we can always link to the
// same static library, regardless of whether the platform
// supports Hailo.

#include "defs.h"

extern "C" {

int cyhailo_load_model(const char* filename, void** model) {
	return cySTATUS_STUBBED;
}
}