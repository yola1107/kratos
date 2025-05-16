package function

import "github.com/jinzhu/copier"

// Copy 深拷贝
func Copy(to any, from any) error {
	return copier.CopyWithOption(to, from, copier.Option{DeepCopy: true})
}
