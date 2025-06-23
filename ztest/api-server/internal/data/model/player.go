package model

import (
	"context"
	"errors"
	"reflect"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type PlayerBaseDataRaw struct {
	UID             int64   `redis:"uid"`
	VIP             int32   `redis:"vip"`
	NickName        string  `redis:"nick"`
	Avatar          string  `redis:"avatar"`
	AvatarUrl       string  `redis:"avatar_url"`
	Money           float64 `redis:"money"`
	ChannelID       int32   `redis:"channel_id"`
	PayTotal        float64 `redis:"pay_total"`
	WithdrawTotal   float64 `redis:"withdraw_total"`
	PresentBet      float64 `redis:"present_bet"`
	PresentProfit   float64 `redis:"present_profit"`
	PresentWinScore float64 `redis:"present_win_score"`
	PresentBoard    int32   `redis:"present_board"`
	TotalBoard      int32   `redis:"total_board"`
	TotalEarn       float64 `redis:"total_earn"`
	TotalConsume    float64 `redis:"total_consume"`
	AllTotalBoard   int64   `redis:"all_total_board"`
}

// SaveStructToRedis saves a struct to Redis using redis tags.
func SaveStructToRedis(ctx context.Context, rdb *redis.Client, key string, v any) error {
	data := StructToRedisMap(v)
	return rdb.HMSet(ctx, key, data).Err()
}

// LoadStructFromRedis loads data from Redis and maps it to a struct using redis tags.
func LoadStructFromRedis[T any](ctx context.Context, rdb *redis.Client, key string) (*T, error) {
	var zero T
	tType := reflect.TypeOf(zero)
	if tType.Kind() != reflect.Struct {
		return nil, errors.New("generic type T must be a struct")
	}
	tPtr := reflect.New(tType)
	tags, tagToPath := extractRedisTags(tPtr.Interface())
	values, err := rdb.HMGet(ctx, key, tags...).Result()
	if err != nil {
		return nil, err
	}
	if err := mapRedisValuesToStruct(tagToPath, values, tPtr.Interface()); err != nil {
		return nil, err
	}
	return tPtr.Interface().(*T), nil
}

func StructToRedisMap(input any) map[string]string {
	result := make(map[string]string)
	v := reflect.ValueOf(input)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	extractStructFields(v, result)
	return result
}

func extractStructFields(v reflect.Value, result map[string]string) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)
		if !fv.CanInterface() {
			continue
		}

		if field.Anonymous || field.Type.Kind() == reflect.Struct {
			extractStructFields(fv, result)
			continue
		}
		tag := field.Tag.Get("redis")
		if tag == "" {
			continue
		}
		if _, exists := result[tag]; exists {
			panic("duplicate redis tag found: " + tag)
		}
		result[tag] = valueToString(fv)
	}
}

func extractRedisTags(input any) ([]string, map[string][]int) {
	tags := []string{}
	tagToPath := make(map[string][]int)

	var walk func(t reflect.Type, path []int)
	walk = func(t reflect.Type, path []int) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("redis")
			p := append(path, i)

			if f.Anonymous || f.Type.Kind() == reflect.Struct {
				walk(f.Type, p)
				continue
			}
			if tag == "" {
				continue
			}
			if _, exists := tagToPath[tag]; exists {
				panic("duplicate redis tag: " + tag)
			}
			tags = append(tags, tag)
			tagToPath[tag] = p
		}
	}

	t := reflect.TypeOf(input)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	walk(t, nil)
	return tags, tagToPath
}

func mapRedisValuesToStruct(tagToPath map[string][]int, values []any, out any) error {
	v := reflect.ValueOf(out).Elem()
	i := 0
	for _, path := range tagToPath {
		if i >= len(values) {
			break
		}
		raw := values[i]
		i++
		if raw == nil {
			continue
		}
		strVal := ""
		switch val := raw.(type) {
		case string:
			strVal = val
		case []byte:
			strVal = string(val)
		default:
			continue
		}
		fv := fieldByPath(v, path)
		if fv.IsValid() && fv.CanSet() {
			assignFieldValue(fv, strVal)
		}
	}
	return nil
}

func fieldByPath(v reflect.Value, path []int) reflect.Value {
	for _, i := range path {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

func assignFieldValue(fv reflect.Value, strVal string) {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(strVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if i64, err := strconv.ParseInt(strVal, 10, 64); err == nil {
			fv.SetInt(i64)
		}
	case reflect.Float32, reflect.Float64:
		if f64, err := strconv.ParseFloat(strVal, 64); err == nil {
			fv.SetFloat(f64)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(strVal); err == nil {
			fv.SetBool(b)
		}
	default:

	}
}

func valueToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	default:
		return ""
	}
}
