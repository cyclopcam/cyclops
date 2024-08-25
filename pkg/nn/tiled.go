package nn

import (
	"fmt"
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
func TiledInference(model ObjectDetector, img ImageCrop, _params *DetectionParams, nThreads int) ([]ObjectDetection, error) {
	config := model.Config()

	// Late clipping seems like a healthy thing, but I haven't verified empirically if it
	// solves any problems yet.
	params := *_params
	params.Unclipped = true

	// This is somewhat arbitrary, and should probably be some multiple of the model size.
	// In practice I think we'll probably restrict model size to something like 1024x1024,
	// which is why I'm not bothering to make this configurable.
	minPadding := 32

	allObjects := []ObjectDetection{}
	allBoxes := []tiledinference.Box{}

	// Note that the CropWidth and CropHeight here will usually be equal to the whole image width and height.
	// The cropping into tiles occurs inside the loop, before we run DetectObject.
	// It would be strange to be running TiledInference on a crop of the image, but we do support that,
	// which is why we start with img.CropWidth, img.CropHeight here.
	// One more thing to note: Our final results are relative to the crop, not of the original 'img'.
	tiling := tiledinference.MakeTiling(img.CropWidth, img.CropHeight, config.Width, config.Height, minPadding)

	tileQueue := make(chan tile, tiling.NumX*tiling.NumY)
	allTiles(tiling, tileQueue)
	//printTiling(tiling)

	//nThreads := runtime.NumCPU()
	//fmt.Printf("Running %v detection threads\n", nThreads)

	detectionResults := make(chan error, nThreads)
	detectionThread := func() {
		for {
			select {
			case tile := <-tileQueue:
				objects, boxes, err := detectTile(model, &params, tiling, tile.x, tile.y, img)
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

	finalClip := Rect{
		X:      0,
		Y:      0,
		Width:  int32(img.CropWidth),
		Height: int32(img.CropHeight),
	}

	if tiling.IsSingle() {
		merged = allObjects

		// We disabled clipping for tiling sake, so we need to clip now
		for i, _ := range merged {
			merged[i].Box = merged[i].Box.Intersection(finalClip)
		}
	} else {
		groups, mergedBoxes := tiledinference.MergeBoxes(tiling, allBoxes, nil)
		for igroup, group := range groups {
			// Start with the first object in the group
			newObj := allObjects[group[0]]
			r := mergedBoxes[igroup]

			// Use the merged box, which can be larger than the first object in the group
			newObj.Box = Rect{X: int32(r.Rect.X1), Y: int32(r.Rect.Y1), Width: int32(r.Rect.Width()), Height: int32(r.Rect.Height())}

			// Clip at the very end, since we disable clipping inside the NN model
			newObj.Box = newObj.Box.Intersection(finalClip)

			// Use max(confidence) from all objects in the group
			for _, el := range group[1:] {
				newObj.Confidence = max(newObj.Confidence, allObjects[el].Confidence)
			}

			merged = append(merged, newObj)
		}
	}

	return merged, nil
}

func printTiling(ti tiledinference.Tiling) {
	for ty := 0; ty < ti.NumY; ty++ {
		for tx := 0; tx < ti.NumX; tx++ {
			r := ti.TileRect(tx, ty)
			fmt.Printf("%v,%v,%v,%v\n", r.X1, r.Y1, r.X2, r.Y2)
		}
	}
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
	tileRect := tiling.TileRect(tx, ty)
	crop := img.Crop(int(tileRect.X1), int(tileRect.Y1), int(tileRect.X2), int(tileRect.Y2))
	//dumpTile(crop)
	objects, err := model.DetectObjects(crop, params)
	if err != nil {
		return nil, nil, err
	}
	boxes := []tiledinference.Box{}
	for i, obj := range objects {
		box := tiledinference.Box{
			Rect: tiledinference.Rect{
				X1: int32(obj.Box.X),
				Y1: int32(obj.Box.Y),
				X2: int32(obj.Box.X + obj.Box.Width),
				Y2: int32(obj.Box.Y + obj.Box.Height),
			},
			Class: int32(obj.Class),
			Tile:  tiling.MakeTileIndex(tx, ty),
		}
		box.Rect.Offset(int32(tileRect.X1), int32(tileRect.Y1))
		objects[i].Box.Offset(int(tileRect.X1), int(tileRect.Y1))
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
