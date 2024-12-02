package nn

import (
	flatbush "github.com/bmharper/flatbush-go"
)

// Scan all pairs of objects in 'input', and if they have a high IoU, and their classes are specified in 'mergeMap',
// then merge them into a single object.
// Returns the list of objects that should be retained.
func MergeSimilarObjects(input []ProcessedObject, mergeMap map[string]string, classes []string, minIoU float32) []int {
	// Create spatial index to avoid O(N^2) comparisons
	fb := flatbush.NewFlatbush[int32]()
	fb.Reserve(len(input))
	for _, b := range input {
		fb.Add(b.Raw.Box.X, b.Raw.Box.Y, b.Raw.Box.X2(), b.Raw.Box.Y2())
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
			for j := range fb.Search(in.Raw.Box.X, in.Raw.Box.Y, in.Raw.Box.X2(), in.Raw.Box.Y2()) {
				if i == j {
					continue
				}
				if deleted[j] {
					continue
				}
				if classes[input[j].Class] != expectOtherClass {
					continue
				}
				if in.Raw.Box.IOU(input[j].Raw.Box) >= minIoU {
					// Delete the class on the 'left' of the map. So if the map says {"truck": "car"},
					// then we delete 'truck' and keep 'car'.
					//fmt.Printf("Deleting %v, and keeping %v\n", classes[in.Class], expectOtherClass)
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

// Scan all pairs of objects in 'input', and if they have a high IoU, and they are abstract objects,
// and their concrete classes differ, then merge them.
// For example:
// A small pickup might get detected by the NN as a "car" and a "truck" with slightly different bounding boxes.
// This will result in two objects getting detected:
// A car and a truck.
// After creating abstract classes, we'll have car, truck, and two vehicles.
// The goal of this function is to squash those two vehicles into a single vehicle.
// Returns the indices of the objects that should be retained.
func MergeSimilarAbstractObjects(input []ProcessedObject, abstractClasses map[int]bool, minIoU float32) []int {
	// Create spatial index to avoid O(N^2) comparisons
	fb := flatbush.NewFlatbush[int32]()
	fb.Reserve(len(input))
	for _, b := range input {
		fb.Add(b.Raw.Box.X, b.Raw.Box.Y, b.Raw.Box.X2(), b.Raw.Box.Y2())
	}
	fb.Finish()

	// The objects that we've already deleted
	deleted := map[int]bool{}
	nChanged := 1

	for nChanged != 0 {
		nChanged = 0
		for i, in := range input {
			if deleted[i] {
				continue
			}
			if _, ok := abstractClasses[in.Class]; !ok {
				continue
			}
			for j := range fb.Search(in.Raw.Box.X, in.Raw.Box.Y, in.Raw.Box.X2(), in.Raw.Box.Y2()) {
				if i == j {
					continue
				}
				if deleted[j] {
					continue
				}
				if _, ok := abstractClasses[input[j].Class]; !ok {
					continue
				}
				if input[j].Raw.Class == in.Raw.Class {
					// The concrete classes must be different (eg truck and car).
					continue
				}
				if in.Raw.Box.IOU(input[j].Raw.Box) >= minIoU {
					// Delete the object 'j' and keep object 'i'
					//fmt.Printf("Deleting %v, and keeping %v\n", input[j].Class, in.Class)
					deleted[j] = true
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
