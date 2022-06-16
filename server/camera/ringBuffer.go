package camera

type RingBuffer[T any] struct {
	MaxSize  int // We guarantee that Size <= MaxSize
	size     int // Current Size
	items    []T
	itemSize []int
	start    int // Items[start] is our first element. If start == len(Items), then we are empty.
}

func (r *RingBuffer[T]) Add(itemSize int, item T) {
	size := r.size
	maxSize := r.MaxSize
	start := r.start
	for ; start < len(r.items) && size+itemSize > maxSize; start++ {
		size -= r.itemSize[start]
	}
	r.start = start

}
