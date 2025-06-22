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

// SaveBaseData saves a struct to Redis
func SaveBaseData(ctx context.Context, rdb *redis.Client, key string, data *PlayerBaseDataRaw) error {
	return SaveStructToRedis(ctx, rdb, key, data)
}

// LoadBaseData loads PlayerBaseDataRaw from Redis
func LoadBaseData(ctx context.Context, rdb *redis.Client, key string) (*PlayerBaseDataRaw, error) {
	result, err := LoadStructFromRedis[PlayerBaseDataRaw](ctx, rdb, key)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SaveStructToRedis saves struct fields to Redis using redis tags.
func SaveStructToRedis(ctx context.Context, rdb *redis.Client, key string, v any) error {
	data := StructToRedisMap(v)
	return rdb.HMSet(ctx, key, data).Err()
}

// LoadStructFromRedis loads data from Redis and maps to a struct using redis tags.
func LoadStructFromRedis[T any](ctx context.Context, rdb *redis.Client, key string) (*T, error) {
	var zero T
	tType := reflect.TypeOf(zero)
	if tType.Kind() != reflect.Struct {
		return nil, errors.New("generic type T must be a struct")
	}

	tPtr := reflect.New(tType)
	fields := extractRedisTags(tPtr.Interface())
	values, err := rdb.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}

	if err := mapRedisValuesToStruct(fields, values, tPtr.Interface()); err != nil {
		return nil, err
	}
	return tPtr.Interface().(*T), nil
}

func extractRedisTags(input any) []string {
	t := reflect.TypeOf(input)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		if tag := t.Field(i).Tag.Get("redis"); tag != "" {
			fields = append(fields, tag)
		}
	}
	return fields
}

func mapRedisValuesToStruct(fields []string, values []any, out any) error {
	v := reflect.ValueOf(out).Elem()
	t := v.Type()

	fieldMap := make(map[string]int)
	for i := 0; i < t.NumField(); i++ {
		if tag := t.Field(i).Tag.Get("redis"); tag != "" {
			fieldMap[tag] = i
		}
	}

	for i, field := range fields {
		raw := values[i]
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

		if fieldIdx, ok := fieldMap[field]; ok {
			fv := v.Field(fieldIdx)
			if !fv.CanSet() {
				continue
			}
			assignFieldValue(fv, strVal)
		}
	}
	return nil
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

func StructToRedisMap(input any) map[string]string {
	result := make(map[string]string)
	v := reflect.ValueOf(input)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("redis")
		if tag == "" {
			continue
		}
		value := v.Field(i)
		// if skipZero && isZero(value) {
		// 	continue
		// }
		result[tag] = valueToString(value)
	}
	return result
}

// func isZero(v reflect.Value) bool {
// 	switch v.Kind() {
// 	case reflect.String:
// 		return v.Len() == 0
// 	case reflect.Bool:
// 		return !v.Bool()
// 	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
// 		return v.Int() == 0
// 	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
// 		return v.Uint() == 0
// 	case reflect.Float32, reflect.Float64:
// 		return v.Float() == 0
// 	case reflect.Ptr, reflect.Interface:
// 		return v.IsNil()
// 	default:
// 		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
// 	}
// }

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
