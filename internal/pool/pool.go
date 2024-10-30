package pool

import (
	"sync"

	"go.uber.org/zap"
)

var fieldPool = &sync.Pool{
	New: func() any {
		return &fieldContainer{
			make([]zap.Field, 0, 32),
		}
	},
}

type fieldContainer struct {
	Fields []zap.Field
}

func (c *fieldContainer) reset() *fieldContainer {
	c.Fields = c.Fields[:0]
	return c
}

// Get selects an arbitrary item from the field Pool, removes it from the
// field Pool, and returns it to the caller.
// Get may choose to ignore the field pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// NOTE: This function should be call Put to give back.
// NOTE: You should know `sync.Pool` work principle
// ```go
//
// fc := logger.Get()
// defer logger.Put(fc)
// fc.Fields = append(fc.Fields, logger.String("k1", "v1"))
// ... use fc.Fields
//
// ```
func Get() *fieldContainer {
	c := fieldPool.Get().(*fieldContainer)
	return c.reset()
}

// Put adds x to the pool.
// NOTE: See Get.
func Put(c *fieldContainer) {
	if c == nil {
		return
	}
	fieldPool.Put(c.reset())
}
