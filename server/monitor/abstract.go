package monitor

func buildMergePairs(classToIdx map[string]int, mergeMap map[string]string) map[uint64]bool {
	result := map[uint64]bool{}
	for left, right := range mergeMap {
		leftIdx := uint64(classToIdx[left])
		rightIdx := uint64(classToIdx[right])
		if leftIdx > 0xffffffff || rightIdx > 0xffffffff {
			panic("Class indices exceed 32 bits")
		}
		if rightIdx < leftIdx {
			leftIdx, rightIdx = rightIdx, leftIdx
		}
		result[(uint64(leftIdx)<<32)|uint64(rightIdx)] = true
	}
	return result
}

func (m *Monitor) makeMergePairKey(classA, classB int) uint64 {
	if classA > classB {
		classA, classB = classB, classA
	}
	return (uint64(classA) << 32) | uint64(classB)
}
