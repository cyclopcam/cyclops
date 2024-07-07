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
	}
	return "Unknown status";
}