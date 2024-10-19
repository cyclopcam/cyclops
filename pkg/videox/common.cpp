#include "common.h"

extern "C" {

// I can't figure out how to get AV_ERROR_MAX_STRING_SIZE into Go code.. so we need this extra malloc
// Note that this means you must free() the result.
char* GetAvErrorStr(int averr) {
	char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, averr);
	return strdup(msg);
}

}