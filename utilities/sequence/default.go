package sequence

var globalSequence = New()

// NextSequence 获取下一个序列值(使用全局序列)
func NextSequence() string {
	return globalSequence.NewSequence()
}
