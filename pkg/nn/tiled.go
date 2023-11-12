package nn

import (
	"os"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/tiledinference"
)

// Run tiled inference on the image.
// We look at the width and height of the model, and if the image is larger, then we split the image
// up into tiles, and run each of those tiles through the model. Then, we merge the tiles back
// into a single dataset.
// If the model is larger than the image, then we just run the model directly, so it is safe
// to call TiledInference on any image, without incurring any performance loss.
func TiledInference(model ObjectDetector, img ImageCrop, params *DetectionParams, nThreads int) ([]ObjectDetection, error) {
	config := model.Config()

	// This is somewhat arbitrary, and should probably be some multiple of the model size.
	// In practice I think we'll probably restrict model size to something like 1024x1024,
	// which is why I'm not bothering to make this configurable.
	minPadding := 64

	allObjects := []ObjectDetection{}
	allBoxes := []tiledinference.Box{}

	// Note that the CropWidth and CropHeight here will usually be equal to the whole image width and height.
	// The cropping into tiles occurs inside the loop, before we run DetectObject.
	tiling := tiledinference.MakeTiling(img.CropWidth, img.CropHeight, config.Width, config.Height, minPadding)

	tileQueue := make(chan tile, tiling.NumX*tiling.NumY)
	allTiles(tiling, tileQueue)

	//nThreads := runtime.NumCPU()
	//fmt.Printf("Running %v detection threads\n", nThreads)

	detectionResults := make(chan error, nThreads)
	detectionThread := func() {
		for {
			select {
			case tile := <-tileQueue:
				objects, boxes, err := detectTile(model, params, tiling, tile.x, tile.y, img)
				if err != nil {
					detectionResults <- err
					return
				}
				allObjects = append(allObjects, objects...)
				allBoxes = append(allBoxes, boxes...)
			default:
				detectionResults <- nil
				return
			}
		}
	}

	for i := 0; i < nThreads; i++ {
		go detectionThread()
	}
	var firstError error
	for i := 0; i < nThreads; i++ {
		err := <-detectionResults
		if err != nil && firstError == nil {
			firstError = err
		}
	}
	if firstError != nil {
		return nil, firstError
	}

	merged := []ObjectDetection{}

	if tiling.IsSingle() {
		merged = allObjects
	} else {
		groups := tiledinference.MergeBoxes(allBoxes, nil)
		for _, group := range groups {
			// Just use the first object in the group
			merged = append(merged, allObjects[group[0]])
		}
	}

	return merged, nil
}

func dumpTile(img ImageCrop) {
	im := cimg.WrapImage(img.ImageWidth, img.ImageHeight, cimg.PixelFormatRGB, img.Pixels)
	crop := cimg.NewImage(img.CropWidth, img.CropHeight, cimg.PixelFormatRGB)
	crop.CopyImageRect(im, img.CropX, img.CropY, img.CropX+img.CropWidth, img.CropY+img.CropHeight, 0, 0)
	b, _ := cimg.Compress(crop, cimg.MakeCompressParams(cimg.Sampling420, 90, 0))
	os.WriteFile("/home/ben/dev/cyclops/foo.jpg", b, 0644)
}

// Returns two parallel arrays
func detectTile(model ObjectDetector, params *DetectionParams, tiling tiledinference.Tiling, tx, ty int, img ImageCrop) ([]ObjectDetection, []tiledinference.Box, error) {
	tx1, ty1, tx2, ty2 := tiling.TileBox(tx, ty)
	crop := img.Crop(tx1, ty1, tx2, ty2)
	//dumpTile(crop)
	objects, err := model.DetectObjects(crop, params)
	if err != nil {
		return nil, nil, err
	}
	boxes := []tiledinference.Box{}
	for i, obj := range objects {
		box := tiledinference.Box{
			X1:    int32(obj.Box.X),
			Y1:    int32(obj.Box.Y),
			X2:    int32(obj.Box.X + obj.Box.Width),
			Y2:    int32(obj.Box.Y + obj.Box.Height),
			Class: int32(obj.Class),
			Tile:  tiling.MakeTileIndex(tx, ty),
		}
		box.Offset(int32(tx1), int32(ty1))
		objects[i].Box.Offset(tx1, ty1)
		boxes = append(boxes, box)
	}
	return objects, boxes, nil
}

type tile struct {
	x int
	y int
}

func allTiles(tiling tiledinference.Tiling, ch chan tile) {
	for ty := 0; ty < tiling.NumY; ty++ {
		for tx := 0; tx < tiling.NumX; tx++ {
			ch <- tile{x: tx, y: ty}
		}
	}
}
