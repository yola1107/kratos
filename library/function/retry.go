package function

// ReTry 重试逻辑，最多重试n次
func ReTry(n int, f func() error) error {
	var err error
	for i := 0; i < n; i++ {
		err = f()
		if err == nil {
			return nil
		}
	}
	return err
}
