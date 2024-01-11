#include <stdint.h>
#include <string.h>

#ifdef __cplusplus
extern "C" {
#endif

size_t EncodeAnnexB(const void* src, size_t srcLen, void* dst, size_t dstLen);
size_t DecodeAnnexB(const void* src, size_t srcLen, void* dst, size_t dstLen);

size_t EncodeAnnexB_Ref(const void* src, size_t srcLen, void* dst, size_t dstLen);
size_t DecodeAnnexB_Ref(const void* src, size_t srcLen, void* dst, size_t dstLen);

#ifdef __cplusplus
}
#endif
