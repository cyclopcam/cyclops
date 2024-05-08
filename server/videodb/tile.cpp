#include "tile.h"

struct TileBuilderX {
};

extern "C" {

TileBuilderX* NewTileBuilderX() {
	return new TileBuilderX();
}

void TileBuilderX_Delete(TileBuilderX* t) {
	delete t;
}
}
