package ext

import (
	"math/rand"
	"sort"

	"golang.org/x/exp/constraints"
)

// 使用标准库约束

type Number interface {
	constraints.Integer | constraints.Float
}

// SliceSum 计算切片中所有元素的累加和
func SliceSum[T Number](data []T) T {
	var sum T
	for _, v := range data {
		sum += v
	}
	return sum
}

// SliceCopy 复制切片
func SliceCopy[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

// SliceShuffle 打乱数据（原地修改）
func SliceShuffle[T any](array []T) {
	rand.Shuffle(len(array), func(i, j int) {
		array[i], array[j] = array[j], array[i]
	})
}

// SliceSort 升序排序（原地修改）
func SliceSort[T Number](slice []T) []T {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	return slice
}

// SliceSortR 降序排序（原地修改）
func SliceSortR[T Number](slice []T) []T {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] > slice[j]
	})
	return slice
}

// SliceIndex 查找值的索引
func SliceIndex[T comparable](data []T, value T) int {
	for i, d := range data {
		if d == value {
			return i
		}
	}
	return -1
}

func SliceDel[T comparable](data []T, values ...T) []T {
	filter := make(map[T]bool, len(values))
	for _, v := range values {
		filter[v] = true
	}
	result := make([]T, 0, len(data))
	for _, v := range data {
		if !filter[v] {
			result = append(result, v)
		}
	}
	return result
}

// SliceDelByIndex 移除指定位置的值
func SliceDelByIndex[T any](data []T, indexes ...int) []T {
	initialSize := len(data)
	if initialSize == 0 {
		return make([]T, 0)
	}

	for i := range indexes {
		if indexes[i] < 0 {
			indexes[i] = initialSize + indexes[i]
		}
	}

	indexes = SliceUniq(indexes)
	sort.Ints(indexes)

	result := make([]T, 0, initialSize)
	result = append(result, data...)

	for i := range indexes {
		if indexes[i]-i < 0 || indexes[i]-i >= initialSize-i {
			continue
		}

		result = append(result[:indexes[i]-i], result[indexes[i]-i+1:]...)
	}

	return result
}

// SliceUniq 切片去重功能
func SliceUniq[T comparable](data []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(data))
	for _, v := range data {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// SliceContains 检查是否包含所有值
func SliceContains[T comparable](data []T, values ...T) bool {
	if len(values) == 0 {
		return true
	}
	valueCount := make(map[T]int)
	for _, v := range values {
		valueCount[v]++
	}
	for _, v := range data {
		if count, exists := valueCount[v]; exists {
			if count > 1 {
				valueCount[v]--
			} else {
				delete(valueCount, v)
			}
		}
		if len(valueCount) == 0 {
			return true
		}
	}
	return len(valueCount) == 0
}

// SliceReverse 翻转切片
func SliceReverse[T any](data []T) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

// SlicePermute 全排列（泛型版）
func SlicePermute[T any](arr []T) [][]T {
	res := [][]T{SliceCopy(arr)}
	cpy := SliceCopy(arr)
	n := len(arr)
	idxs := make([]int, n)

	i := 0
	for i < n {
		if idxs[i] < i {
			if i%2 == 0 {
				cpy[0], cpy[i] = cpy[i], cpy[0]
			} else {
				cpy[idxs[i]], cpy[i] = cpy[i], cpy[idxs[i]]
			}
			res = append(res, SliceCopy(cpy))
			idxs[i]++
			i = 0
		} else {
			idxs[i] = 0
			i++
		}
	}
	return res
}

// 辅助函数：计算阶乘
func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n-1)
}

// SliceCartesian  笛卡尔积（泛型版）
func SliceCartesian[T any](items ...[]T) [][]T {
	if len(items) == 0 {
		return nil
	}

	// 预计算结果容量
	capacity := 1
	for _, s := range items {
		capacity *= len(s)
	}
	result := make([][]T, 0, capacity) // 预分配

	var backtrack func(int, []T)
	backtrack = func(idx int, path []T) {
		if idx == len(items) {
			result = append(result, SliceCopy(path))
			return
		}
		for _, v := range items[idx] {
			backtrack(idx+1, append(path, v))
		}
	}

	backtrack(0, make([]T, 0, len(items)))
	return result
}

// SliceForEach 遍历切片
func SliceForEach[T any](data []T, fn func(item T, index int)) {
	for i, v := range data {
		fn(v, i)
	}
}

// SliceMap 映射切片
func SliceMap[T any, R any](data []T, fn func(item T, index int) R) []R {
	result := make([]R, len(data))
	for i := range data {
		result[i] = fn(data[i], i)
	}
	return result
}

// SliceReduce 归约切片
func SliceReduce[T any, R any](data []T, fn func(agg R, item T, index int) R, initial R) R {
	for i := range data {
		initial = fn(initial, data[i], i)
	}
	return initial
}

func SliceFilter[T any, Slice ~[]T](data Slice, fn func(item T, index int) bool) Slice {
	result := make(Slice, 0, len(data))
	for i := range data {
		if fn(data[i], i) {
			result = append(result, data[i])
		}
	}
	return result
}
