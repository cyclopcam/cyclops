#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef void* NcnnDetector;

NcnnDetector CreateDetector(const char* type, const char* param, const char* bin);
void         DeleteDetector(NcnnDetector detector);

#ifdef __cplusplus
}
#endif
