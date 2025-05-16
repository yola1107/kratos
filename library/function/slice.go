package function

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

func SliceRemove[T comparable](data []T, values ...T) []T {
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

// SliceRemoveByIndex 移除指定位置的值
func SliceRemoveByIndex[T any](data []T, index int) []T {
	if index < 0 || index >= len(data) {
		return data
	}
	return append(data[:index], data[index+1:]...)
}

// SliceRemoveFirstValue 移除第一个指定值
func SliceRemoveFirstValue[T comparable](data []T, value T) []T {
	for i, d := range data {
		if d == value {
			return append(data[:i], data[i+1:]...)
		}
	}
	return data
}

// SliceValueIndex 查找值的索引
func SliceValueIndex[T comparable](data []T, value T) int {
	for i, d := range data {
		if d == value {
			return i
		}
	}
	return -1
}

// SliceUnique 切片去重功能
func SliceUnique[T comparable](data []T) []T {
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

// SliceContain 检查是否包含值
func SliceContain[T comparable](data []T, value T) bool {
	for _, v := range data {
		if v == value {
			return true
		}
	}
	return false
}

// SliceContainAll 检查是否包含所有值
func SliceContainAll[T comparable](data []T, values ...T) bool {
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

// SlicePick 挑选指定索引的值
func SlicePick[T any](data []T, indexes []int) []T {
	var result []T
	dataLen := len(data)
	for _, index := range indexes {
		if index >= 0 && index < dataLen {
			result = append(result, data[index])
		}
	}
	return result
}

// SliceRandN 随机选择n个元素
func SliceRandN[T any](array []T, n int) []T {
	if n <= 0 || len(array) == 0 {
		return nil
	}
	tmp := make([]T, len(array))
	copy(tmp, array)
	SliceShuffle(tmp)
	if n >= len(tmp) {
		return tmp
	}
	return tmp[:n]
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

// SliceDirect 笛卡尔积（泛型版）
func SliceDirect[T any](items ...[]T) [][]T {
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
func SliceForEach[T any](data []T, fn func(int, T)) {
	for i, v := range data {
		fn(i, v)
	}
}

// SliceMap 映射切片
func SliceMap[T any, Y any](data []T, fn func(int, T) Y) []Y {
	result := make([]Y, len(data))
	for i, v := range data {
		result[i] = fn(i, v)
	}
	return result
}

// SliceReduce 归约切片
func SliceReduce[T any, R any](data []T, fn func(R, T) R, init R) R {
	acc := init
	for _, v := range data {
		acc = fn(acc, v)
	}
	return acc
}
