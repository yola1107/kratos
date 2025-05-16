package ext

// MapKeys 返回 map 的所有键（无序）
// 注意：返回键的顺序不固定，与 map 遍历顺序一致
// 当 data 为 nil 时返回 nil
func MapKeys[K comparable, V any](data map[K]V) []K {
	if data == nil {
		return nil
	}
	keys := make([]K, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}

// MapValues 返回 map 的所有 value
func MapValues[K comparable, V any](data map[K]V) []V {
	values := make([]V, 0, len(data))
	for _, v := range data {
		values = append(values, v)
	}
	return values
}

// MapKeysSum 计算 map 所有键的累加和
// 当 data 为 nil 或空时返回类型零值
func MapKeysSum[K Number, V any](data map[K]V) K {
	var sum K
	for k := range data {
		sum += k
	}
	return sum
}

// MapValuesSum 计算 map 所有值的累加和
// 当 data 为 nil 或空时返回类型零值
func MapValuesSum[K comparable, V Number](data map[K]V) V {
	var sum V
	for _, v := range data {
		sum += v
	}
	return sum
}

// MapForEach 遍历 map 并执行操作
// 注意：遍历顺序不固定，与 map 迭代顺序一致
func MapForEach[K comparable, V any](data map[K]V, fn func(K, V)) {
	for k, v := range data {
		fn(k, v)
	}
}

// MapReduce 对 map 进行归约计算
// init: 初始累积值
// fn: 接收当前键、值和累积值，返回新累积值的函数
func MapReduce[K comparable, V any, R any](
	data map[K]V,
	fn func(K, V, R) R,
	init R,
) R {
	for k, v := range data {
		init = fn(k, v, init)
	}
	return init
}

// MapToMap 转换 map 的键值类型（可能发生键覆盖）
// 重要：当不同键转换为相同新键时，最后一个值会保留
// 当 data 为 nil 时返回 nil
func MapToMap[K1, K2 comparable, V1, V2 any](
	data map[K1]V1,
	fn func(K1, V1) (K2, V2),
) map[K2]V2 {
	if data == nil {
		return nil
	}
	result := make(map[K2]V2, len(data))
	for k, v := range data {
		k2, v2 := fn(k, v)
		result[k2] = v2
	}
	return result
}
