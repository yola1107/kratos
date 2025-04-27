package ext

import "github.com/jinzhu/copier"

// Copy 浅拷贝
func Copy(to, from any) error {
	return copier.CopyWithOption(to, from, copier.Option{DeepCopy: false})
}

// DeepCopy 深拷贝
func DeepCopy(to, from any) error {
	return copier.CopyWithOption(to, from, copier.Option{DeepCopy: true})
}
