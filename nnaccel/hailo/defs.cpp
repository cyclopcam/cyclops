#include "defs.h"

const char* _cyhailo_status_str_own(cyStatus s) {
	switch (s) {
	case cySTATUS_OK:
		return "OK";
	case cySTATUS_STUBBED:
		return "Hailo support has been stubbed out, because we could't find the hailort include files such as 'hailort.h'";
	case cySTATUS_MODEL_NOT_LOADED:
		return "Model not loaded";
	case cySTATUS_INVALID_INPUT_DIMENSIONS:
		return "Invalid input dimensions";
	case cySTATUS_OUT_OF_CPU_MEMORY:
		return "Out of CPU memory";
	case cySTATUS_TIMEOUT:
		return "Timeout";
	case cySTATUS_CPU_NMS_NOT_IMPLEMENTED:
		return "CPU NMS not implemented";
	case cySTATUS_SPARSE_SCANLINES:
		return "Scanlines are not densely packed. Stride must be nchan*width for batch sizes other than 1";
	default:
		return "Unknown status";
	}
}