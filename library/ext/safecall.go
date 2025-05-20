package ext

// SafeCall 安全执行回调
func SafeCall(fn func()) {
	defer RecoverFromError(nil)
	if fn != nil {
		fn()
	}
}

// SafeSyncCall 异步安全执行回调
func SafeSyncCall(fn func()) {
	go func() {
		SafeCall(fn)
	}()
}
