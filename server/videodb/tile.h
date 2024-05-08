#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct TileBuilderX TileBuilderX;

TileBuilderX* NewTileBuilderX();
void          TileBuilderX_Delete(TileBuilderX* t);

#ifdef __cplusplus
}
#endif
