package ext

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

/*
	"github.com/samber/lo"

	samber/lo 是一个非常流行的 Go 语言库，它提供了一些常用的函数式编程风格的工具函数，使得 Go 代码更加简洁、优雅。该库的设计理念是减少代码冗余，简化开发过程，尤其是在处理常见的数据结构和算法时。

	lo 这个库的名字来源于 "Lazily Optimized" 的缩写，但它本身并不涉及延迟计算或优化策略。它的目标是提供一系列常见操作的简便方法，尤其是集合类型（如切片、映射、通道等）的操作。

*/

func TestSliceReduce(t *testing.T) {
	t.Run("Reduce_Sum", func(t *testing.T) {
		sum := SliceReduce([]int32{1, 2, 3}, func(agg int32, item int32, index int) int32 {
			return agg + item
		}, 0)
		require.Equal(t, int32(6), sum)
	})

	t.Run("Filter_EvenIndex", func(t *testing.T) {
		out := SliceFilter([]int32{1, 2, 3}, func(item int32, index int) bool {
			return index%2 == 0
		})
		require.Equal(t, []int32{1, 3}, out)
	})

	t.Run("To_Map", func(t *testing.T) {
		out := SliceMap([]int32{1, 2, 3}, func(item int32, index int) string {
			return fmt.Sprintf("{k=%d,v=%v}", index, item)
		})
		require.Equal(t, []string{"{k=0,v=1}", "{k=1,v=2}", "{k=2,v=3}"}, out)
	})

}

func TestLo_BasicFunction(t *testing.T) {
	t.Run("Reduce_Sum", func(t *testing.T) {
		sum := lo.Reduce([]int32{1, 2, 3}, func(agg, v int32, _ int) int32 {
			return agg + v
		}, 0)
		require.Equal(t, int32(6), sum)
	})

	t.Run("Filter_EvenIndex", func(t *testing.T) {
		out := lo.Filter([]int32{1, 2, 3}, func(v int32, i int) bool {
			return i%2 == 0
		})
		require.Equal(t, []int32{1, 3}, out)
	})

	t.Run("Map_SquareToString", func(t *testing.T) {
		out := lo.Map([]int32{1, 2, 3}, func(v int32, _ int) string {
			return fmt.Sprintf("%d", v*v)
		})
		require.Equal(t, []string{"1", "4", "9"}, out)
	})

	t.Run("Shuffle_LengthUnchanged", func(t *testing.T) {
		in := []int32{1, 2, 3, 4, 5}
		out := lo.Shuffle(in)
		require.Len(t, out, len(in))
	})

	t.Run("Duration_Sleep1s", func(t *testing.T) {
		dur := lo.Duration(func() {
			time.Sleep(50 * time.Millisecond)
		})
		require.GreaterOrEqual(t, dur, 50*time.Millisecond)
	})

	t.Run("Async1_ReturnsAAA", func(t *testing.T) {
		t.Parallel()
		ch := lo.Async1(func() any {
			time.Sleep(10 * time.Millisecond)
			return "aaa"
		})
		require.Equal(t, "aaa", <-ch)
	})

	t.Run("ForEach_PrintCheck", func(t *testing.T) {
		// This test just verifies no panic. You may capture stdout if needed.
		lo.ForEach([]int32{1, 2, 3}, func(v int32, i int) {
			_ = v + int32(i)
		})
	})

	t.Run("GroupBy_OddEven", func(t *testing.T) {
		out := lo.GroupBy([]int{1, 2, 3, 4, 5, 6}, func(v int) string {
			if v%2 == 0 {
				return "even"
			}
			return "odd"
		})
		require.ElementsMatch(t, out["even"], []int{2, 4, 6})
		require.ElementsMatch(t, out["odd"], []int{1, 3, 5})
	})

	t.Run("Uniq_RemoveDuplicates", func(t *testing.T) {
		out := lo.Uniq([]int{1, 3, 2, 3, 1, 2, 3})
		require.ElementsMatch(t, out, []int{1, 3, 2})
	})

	t.Run("IndexOf_ValueExists", func(t *testing.T) {
		idx := lo.IndexOf([]int{1, 2, 3, 4}, 3)
		require.Equal(t, 2, idx)
	})

	t.Run("Contains_ValueCheck", func(t *testing.T) {
		require.True(t, lo.Contains([]int{1, 2, 3, 4}, 3))
	})

	//
	t.Run("Times_GenerateN", func(t *testing.T) {
		out := lo.Times(5, func(i int) int {
			return i * i
		})
		require.Equal(t, []int{0, 1, 4, 9, 16}, out)
	})

	t.Run("Reverse_Order", func(t *testing.T) {
		out := lo.Reverse([]int{1, 2, 3, 4})
		require.Equal(t, []int{4, 3, 2, 1}, out)
	})

	t.Run("Chunk_SplitSlice", func(t *testing.T) {
		out := lo.Chunk([]int{1, 2, 3, 4, 5}, 2)
		require.Equal(t, [][]int{{1, 2}, {3, 4}, {5}}, out)
	})

	t.Run("Flatten_NestedSlice", func(t *testing.T) {
		nested := [][]int{{1, 2}, {3}, {4, 5}}
		out := lo.Flatten(nested)
		require.Equal(t, []int{1, 2, 3, 4, 5}, out)
	})

	t.Run("Sample_AnyItem", func(t *testing.T) {
		val := lo.Sample([]string{"a", "b", "c"})
		require.Contains(t, []string{"a", "b", "c"}, val)
	})

	t.Run("Samples_AnyItem", func(t *testing.T) {
		val := lo.Samples([]string{"a", "b", "c"}, 2)
		for _, v := range val {
			require.Contains(t, []string{"a", "b", "c"}, v)
		}
	})

	t.Run("Intersect_CommonValues", func(t *testing.T) {
		out := lo.Intersect([]int{1, 2, 3}, []int{2, 3, 4})
		require.ElementsMatch(t, []int{2, 3}, out)
	})

	t.Run("Difference_Remaining", func(t *testing.T) {
		out1, out2 := lo.Difference([]int{1, 2, 3}, []int{2, 3, 4})
		require.Equal(t, []int{1}, out1)
		require.Equal(t, []int{4}, out2)
	})

	t.Run("Compact_RemoveZero", func(t *testing.T) {
		out := lo.Compact([]int{0, 1, 2, 0, 3})
		require.Equal(t, []int{1, 2, 3}, out)
	})

	t.Run("Must_NoError", func(t *testing.T) {
		val := lo.Must(func() (int, error) {
			return 10, nil
		}())
		require.Equal(t, 10, val)
	})

	// 3. lo.Keys, lo.Values, lo.Associate
	t.Run("KeysAndValues", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		require.ElementsMatch(t, []string{"a", "b"}, lo.Keys(m))
		require.ElementsMatch(t, []int{1, 2}, lo.Values(m))
	})

	// 4. lo.Range
	t.Run("RangeFromToStep", func(t *testing.T) {
		out := lo.RangeFrom(5, 3) // [5,6,7]
		require.Equal(t, []int{5, 6, 7}, out)
	})

	// 5. lo.PickBy, lo.Reject
	t.Run("PickBy_OnlyEven", func(t *testing.T) {
		m := map[int]string{1: "a", 2: "b", 3: "c", 4: "d"}
		evenMap := lo.PickBy(m, func(k int, _ string) bool { return k%2 == 0 })
		require.Equal(t, map[int]string{2: "b", 4: "d"}, evenMap)
	})

	// 7. lo.Must0, lo.Must1, lo.Must2（panic捕获）
	t.Run("Must2_ValidResult", func(t *testing.T) {
		a, b := lo.Must2(func() (int, string, error) {
			return 1, "ok", nil
		}())
		require.Equal(t, 1, a)
		require.Equal(t, "ok", b)
	})

	// 8. lo.IsEmpty, lo.IsNotEmpty
	t.Run("IsEmptyAndNotEmpty", func(t *testing.T) {
		require.True(t, lo.IsEmpty(0))
		require.False(t, lo.IsNotEmpty(0))
		require.True(t, lo.IsNotEmpty(1))
	})

	// 9. lo.Try, lo.TryCatch
	t.Run("TryCatch_NoPanic", func(t *testing.T) {
		var ok bool
		lo.TryCatch(func() error {
			_ = 1 + 2
			ok = true
			return nil
		}, func() {
			t.Fatal("unexpected panic")
		})
		require.True(t, ok)
	})

	t.Run("TryCatch_WithPanic", func(t *testing.T) {
		var recovered bool
		lo.TryCatch(func() error {
			panic("something went wrong")
			return nil
		}, func() {
			recovered = true
		})
		require.True(t, recovered)
	})
}

func TestUserOperationsWithLo(t *testing.T) {
	type User struct {
		ID    int
		Name  string
		Age   int
		Email string
	}

	var users = []User{
		{ID: 1, Name: "Alice", Age: 25, Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Age: 30, Email: "bob@example.com"},
		{ID: 3, Name: "Charlie", Age: 18, Email: "charlie@example.com"},
		{ID: 4, Name: "Alice", Age: 25, Email: "alice2@example.com"}, // 重名
	}

	t.Run("Filter_Adults", func(t *testing.T) {
		adults := lo.Filter(users, func(u User, _ int) bool {
			return u.Age >= 21
		})
		require.Len(t, adults, 3)
	})

	t.Run("Map_ExtractNames", func(t *testing.T) {
		names := lo.Map(users, func(u User, _ int) string {
			return u.Name
		})
		require.Equal(t, []string{"Alice", "Bob", "Charlie", "Alice"}, names)
	})

	t.Run("UniqBy_Name", func(t *testing.T) {
		uniqueNames := lo.UniqBy(users, func(u User) string {
			return u.Name
		})
		require.Len(t, uniqueNames, 3)
	})

	t.Run("GroupBy_Name", func(t *testing.T) {
		grouped := lo.GroupBy(users, func(u User) string {
			return u.Name
		})
		require.Equal(t, 3, len(grouped)) // Alice, Bob, Charlie
		require.Len(t, grouped["Alice"], 2)
	})

	t.Run("Reduce_AgeSum", func(t *testing.T) {
		sum := lo.Reduce(users, func(agg int, u User, _ int) int {
			return agg + u.Age
		}, 0)
		require.Equal(t, 98, sum)
	})

	t.Run("FindBy_Email", func(t *testing.T) {
		found, ok := lo.Find(users, func(u User) bool {
			return u.Email == "charlie@example.com"
		})
		require.True(t, ok)
		require.Equal(t, "Charlie", found.Name)
	})

	t.Run("ContainsBy_IDCheck", func(t *testing.T) {
		exists := lo.ContainsBy(users, func(u User) bool {
			return u.ID == 2
		})
		require.True(t, exists)
	})
}

func TestCopy(t *testing.T) {
	a := 1
	b := 2
	log.Printf("%p %p\n", &a, &b)
	_ = DeepCopy(&b, &a)
	log.Printf("%p %v\n", &a, a)
	log.Printf("%p %v\n", &b, b)
}

func TestDiff(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"b", "a", "c"}
	b = lo.Shuffle(b)
	change, err := Diff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(change)
}
