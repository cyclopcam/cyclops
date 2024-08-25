package nn

import (
	"fmt"

	flatbush "github.com/bmharper/flatbush-go"
)

// Scan all pairs of objects in 'input', and if they have a high IoU, and their classes are specified in 'mergeMap',
// then merge them into a single object.
// Returns the list of objects that should be retained.
func MergeSimilarObjects(input []ObjectDetection, mergeMap map[string]string, classes []string, minIoU float32) []int {
	// Create spatial index to avoid O(N^2) comparisons
	fb := flatbush.NewFlatbush[int32]()
	fb.Reserve(len(input))
	for _, b := range input {
		fb.Add(b.Box.X, b.Box.Y, b.Box.X2(), b.Box.Y2())
	}
	fb.Finish()

	// The objects that we've already merged
	deleted := map[int]bool{}
	nChanged := 1

	for nChanged != 0 {
		nChanged = 0
		for i, in := range input {
			if deleted[i] {
				continue
			}
			expectOtherClass, ok := mergeMap[classes[in.Class]]
			if !ok {
				continue
			}
			for j := range fb.Search(in.Box.X, in.Box.Y, in.Box.X2(), in.Box.Y2()) {
				if i == j {
					continue
				}
				if deleted[j] {
					continue
				}
				if classes[input[j].Class] != expectOtherClass {
					continue
				}
				if in.Box.IOU(input[j].Box) >= minIoU {
					// Delete the class on the 'left' of the map. So if the map says {"truck": "car"},
					// then we delete 'truck' and keep 'car'.
					fmt.Printf("Deleting %v, and keeping %v\n", classes[in.Class], expectOtherClass)
					deleted[i] = true
					nChanged++
				}
			}
		}
	}

	retain := make([]int, 0, len(input))
	for i := range input {
		if !deleted[i] {
			retain = append(retain, i)
		}
	}
	return retain
}
